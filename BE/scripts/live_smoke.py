#!/usr/bin/env python3
"""Opt-in real-provider generation probe against a running backend. No API keys read here."""
import argparse,json,time,uuid,urllib.request,urllib.error,pathlib,os,getpass
from datetime import date,timedelta
def main():
    ap=argparse.ArgumentParser();ap.add_argument('--url',default='http://127.0.0.1:8080');ap.add_argument('--days',type=int,default=1);ap.add_argument('--transport',choices=('walking','transit'),default='walking');ap.add_argument('--revise',action='store_true');ap.add_argument('--revision-scope',choices=('afternoon','day'),default='afternoon');ap.add_argument('--email',required=True,help='existing verified account email');args=ap.parse_args();token=None
    def call(method,path,body=None,key=None,want=200):
        headers={'Content-Type':'application/json'};data=None if body is None else json.dumps(body).encode()
        if token:headers['Authorization']='Bearer '+token
        if key:headers['Idempotency-Key']=key
        try:
            with urllib.request.urlopen(urllib.request.Request(args.url+path,data=data,method=method,headers=headers),timeout=12) as r:status=r.status;out=json.load(r)
        except urllib.error.HTTPError as e:status=e.code;out=json.load(e)
        if status!=want:raise RuntimeError(json.dumps({'http_status':status,'code':out.get('code'),'message':out.get('message')},ensure_ascii=False))
        return out.get('data',out)
    def poll(j):
        end=time.monotonic()+650;last=None
        while time.monotonic()<end:
            out=call('GET','/api/v1/planning-jobs/'+j['id']);state=(out['status'],out['stage'])
            if state!=last:print(json.dumps({'job_id':j['id'],'status':state[0],'stage':state[1]},ensure_ascii=False),flush=True);last=state
            if out['status'] in ('succeeded','failed','conflicted','interrupted'):return out
            time.sleep(1)
        raise RuntimeError('polling timed out')
    for _ in range(60):
        try:call('GET','/readyz');break
        except Exception:time.sleep(.5)
    else:raise RuntimeError('backend not ready')
    password=os.environ.get('TRAVEL_LOGIN_PASSWORD') or getpass.getpass('Account password: ')
    tag=uuid.uuid4().hex[:10];token=call('POST','/api/v1/auth/login',{'email':args.email,'password':password})['token'];start=date(2026,10,1)
    constraints={'city':'杭州市','start_date':str(start),'end_date':str(start+timedelta(days=args.days-1)),'party_size':2,'budget_cents':500000,'budget_scope':'total','interests':['文化','自然风景'],'pace':'relaxed','transport':args.transport}
    begin=time.monotonic();j=poll(call('POST','/api/v1/planning-jobs',{'constraints':constraints},key='live-'+tag,want=202));result={'provider_mode':'real','days':args.days,'transport':args.transport,'status':j['status'],'elapsed_seconds':round(time.monotonic()-begin,2),'model_runs':j.get('model_runs'),'error':j.get('error')}
    if j['status']=='succeeded':
        trip=call('GET','/api/v1/trips/'+j['trip_id']);budget=call('GET','/api/v1/trips/'+j['trip_id']+'/budget');result.update({'trip_id':trip['id'],'version':trip['version'],'activity_count':len(trip['plan']['activities']),'route_count':len(trip['plan']['routes']),'known_total_cents':budget['known_total_cents'],'unknown_count':budget['unknown_count']})
        if args.revise:
            scope_date=str(start+timedelta(days=1 if args.days>=2 else 0));window_start,window_end=(0,1440) if args.revision_scope=='day' else (720,1200)
            selected=[a for a in trip['plan']['activities'] if a['date']==scope_date and a['start_minute']>=window_start and a['end_minute']<=window_end]
            if not any(a['kind']=='sightseeing' for a in selected):window_start,window_end=0,1440;selected=[a for a in trip['plan']['activities'] if a['date']==scope_date]
            assert any(a['kind']=='sightseeing' for a in selected), 'revision day has no sightseeing activity'
            selected_ids={a['id'] for a in selected};scope={'date':scope_date,'start_minute':window_start,'end_minute':window_end,'editable_item_ids':sorted(selected_ids),'locked_item_ids':[]};changed=poll(call('POST','/api/v1/trips/'+trip['id']+'/replanning-jobs',{'expected_version':trip['version'],'scope':scope,'instruction':('将这一天调整为室内博物馆主题，保留必要餐饮和住宿，其他日期保持不变' if args.revision_scope=='day' else '将这个下午调整为室内博物馆活动，并安排必要餐饮，其他安排保持不变')},key='revise-'+tag,want=202));result['revision']={'requested_scope_kind':args.revision_scope,'scope_kind':('day' if (window_start,window_end)==(0,1440) else 'afternoon'),'date':scope_date,'start_minute':window_start,'end_minute':window_end,'status':changed['status'],'version':changed.get('result_version'),'error':changed.get('error'),'model_runs':changed.get('model_runs')}
            if changed['status']=='succeeded':
                current=call('GET','/api/v1/trips/'+trip['id']);after={a['id']:a for a in current['plan']['activities']}
                assert current['version']==trip['version']+1 and selected_ids.isdisjoint(after)
                for a in trip['plan']['activities']:
                    if a['id'] not in selected_ids:assert after[a['id']]==a
                replacement=[a for a in current['plan']['activities'] if a['id'] not in {x['id'] for x in trip['plan']['activities']}]
                assert replacement, 'revision succeeded without a replacement activity'
                for a in replacement:
                    assert a['date']==scope['date'], ('replacement date outside scope',a)
                    assert scope['start_minute']<=a['start_minute']<a['end_minute']<=scope['end_minute'], ('replacement time outside scope',a)
                    assert a['id'] not in {x['id'] for x in trip['plan']['activities']}, ('replacement reused old activity id',a)
                assert any(a['kind']=='sightseeing' and ('博物馆' in a.get('place',{}).get('name','') or '博物馆' in a.get('place',{}).get('tags',[])) for a in replacement)
                result['revision']['outside_activities_preserved']=True
    report=pathlib.Path(__file__).resolve().parents[1]/'.local'/('live-'+tag+'.json');report.parent.mkdir(exist_ok=True);report.write_text(json.dumps(result,ensure_ascii=False,indent=2));result['report_path']=str(report)
    call('POST','/api/v1/auth/logout');print(json.dumps(result,ensure_ascii=False,indent=2),flush=True)
    if result['status']!='succeeded' or (args.revise and result.get('revision',{}).get('status')!='succeeded'):raise SystemExit(1)
if __name__=='__main__':main()
