#!/usr/bin/env python3
"""End-to-end test: real HTTP, Kitex, PostgreSQL; explicit fake external providers."""
import argparse,json,os,subprocess,time,urllib.request,urllib.error,uuid,pathlib,signal
from smtp_fixture import Mailbox
ROOT=pathlib.Path(__file__).resolve().parents[1]
def run():
    ap=argparse.ArgumentParser();ap.add_argument('--database-url',default=os.environ.get('TEST_DATABASE_URL',''));args=ap.parse_args()
    if not args.database_url:raise SystemExit('Set TEST_DATABASE_URL to a dedicated PostgreSQL database')
    processes=[];logs=[];checks=[];base='http://127.0.0.1:18080';tag=uuid.uuid4().hex[:8];logdir=ROOT/'.local'/f'smoke-{tag}';logdir.mkdir(parents=True)
    env=dict(os.environ,DATABASE_URL=args.database_url,PLANNER_SERVICE_TOKEN=uuid.uuid4().hex,AMAP_API_KEY='fixture-map-key',BIGMODEL_API_KEY='fixture-model-key',ALLOW_LOCAL_PROVIDERS='true',AMAP_BASE_URL='http://127.0.0.1:18081',BIGMODEL_BASE_URL='http://127.0.0.1:18081',API_ADDR='127.0.0.1:18080',TRAVEL_ADDR='127.0.0.1:18888',PLANNER_ADDR='127.0.0.1:18889',OPENAPI_PATH=str(ROOT/'docs/openapi.yaml'))
    mailbox=Mailbox(logdir/'mail')
    env.update(mailbox.environment());env['AUTH_CODE_HMAC_KEY']=uuid.uuid4().hex+uuid.uuid4().hex
    def launch(name,cmd):
        f=(logdir/f'{name}.log').open('w');logs.append(f);p=subprocess.Popen(cmd,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT);processes.append(p);return p
    def call(method,path,body=None,token=None,key=None,want=200):
        headers={};data=None
        if body is not None:data=json.dumps(body).encode();headers['Content-Type']='application/json'
        if token:headers['Authorization']='Bearer '+token
        if key:headers['Idempotency-Key']=key
        req=urllib.request.Request(base+path,data=data,method=method,headers=headers)
        try:
            with urllib.request.urlopen(req,timeout=18) as r:status=r.status;payload=json.load(r)
        except urllib.error.HTTPError as e:status=e.code;payload=json.load(e)
        if status!=want:raise AssertionError(f'{method} {path}: HTTP {status} expected {want}; code={payload.get("code")}')
        return payload.get('data',payload)
    def poll(j,token):
        for _ in range(240):
            v=call('GET','/api/v1/planning-jobs/'+j['id'],token=token)
            if v['status'] in ('succeeded','failed','conflicted','interrupted'):
                if v['status']!='succeeded':raise AssertionError(f'job {v["status"]}: {v.get("error")}')
                return v
            time.sleep(.25)
        raise AssertionError('job polling timed out')
    try:
        launch('fixtures',['python3','scripts/fixture_provider.py']);launch('planner',['bin/planner']);travel=launch('travel',['bin/travel']);launch('api',['bin/api'])
        for _ in range(100):
            try:call('GET','/readyz');break
            except Exception:time.sleep(.2)
        else:raise AssertionError('readiness failed; see '+str(logdir))
        def register_email(email,password):
            after=len(mailbox.messages);challenge=call('POST','/api/v1/auth/register/code',{'email':email});mail=mailbox.wait(email,after)
            assert 'code' not in challenge and challenge['retry_after']==60
            proof={'email':email,'password':password,'code':mail['code'],'challenge_id':challenge['challenge_id']}
            wrong=dict(proof,code='000000' if mail['code']!='000000' else '111111')
            call('POST','/api/v1/auth/register',wrong,want=400)
            user=call('POST','/api/v1/auth/register',proof,want=201);assert user['email_verified'] and user['email']==email
            call('POST','/api/v1/auth/register',proof,want=400)
            call('POST','/api/v1/auth/register/code',{'email':email},want=429)
            return call('POST','/api/v1/auth/login',{'email':email,'password':password})
        name='smoke_'+tag+'@example.test';pw=uuid.uuid4().hex;session=register_email(name,pw);token=session['token'];checks.append('email verification over real TLS SMTP + HTTP/Kitex/PostgreSQL, wrong-code/replay/cooldown checks')
        assert call('GET','/api/v1/me',token=token)['email']==name
        checks.append('email/password login and verified user profile')
        places=call('GET','/api/v1/places?city=%E6%9D%AD%E5%B7%9E%E5%B8%82',token=token);assert places['items'];checks.append('provider POI through HTTP/RPC')
        request={'constraints':{'city':'杭州市','start_date':'2026-10-01','end_date':'2026-10-03','party_size':2,'budget_cents':200000,'budget_scope':'per_person','interests':['文化'],'pace':'balanced','transport':'walking'}}
        j=call('POST','/api/v1/planning-jobs',request,token,'create-'+tag,202);duplicate=call('POST','/api/v1/planning-jobs',request,token,'create-'+tag,202);assert duplicate['id']==j['id'];checks.append('idempotent submission')
        done=poll(j,token);trip_id=done['trip_id'];trip=call('GET','/api/v1/trips/'+trip_id,token=token);assert trip['version']==1 and len(trip['plan']['routes'])==3;checks.append('three-day generation + persisted real RPC pipeline')
        budget=call('GET','/api/v1/trips/'+trip_id+'/budget',token=token);assert budget['budget_total_cents']==400000 and budget['known_total_cents']==96000 and not budget['complete'];checks.append('budget conversion and incomplete costs')
        target=next(a for a in trip['plan']['activities'] if a['date']=='2026-10-02' and a['start_minute']==840)
        edit={'expected_version':1,'scope':{'date':'2026-10-02','start_minute':840,'end_minute':960,'editable_item_ids':[target['id']],'locked_item_ids':[]},'instruction':'第二天下午改为室内活动'}
        revised=poll(call('POST','/api/v1/trips/'+trip_id+'/replanning-jobs',edit,token,'revise-'+tag,202),token);assert revised['result_version']==2
        current=call('GET','/api/v1/trips/'+trip_id,token=token);original={a['id']:a for a in trip['plan']['activities']};after={a['id']:a for a in current['plan']['activities']};assert target['id'] not in after
        for aid,a in original.items():
            if aid!=target['id']:assert after[aid]==a
        assert call('GET','/api/v1/trips/'+trip_id+'/versions/1',token=token)['plan']==trip['plan'];checks.append('scoped replan preserves outside activities + immutable history')
        allowed=('id','date','start_minute','end_minute','kind','place_id','title','reason');manual={'expected_version':2,'plan':{'title':'手动标题','summary':current['plan']['summary'],'activities':[{k:a[k] for k in allowed} for a in current['plan']['activities']]}}
        edited=poll(call('PATCH','/api/v1/trips/'+trip_id,manual,token,'manual-'+tag,202),token);assert edited['result_version']==3 and not edited.get('model_runs');checks.append('manual edit without model')
        call('POST','/api/v1/trips/'+trip_id+'/replanning-jobs',edit,token,'stale-'+tag,409);checks.append('stale expected_version rejected')
        other='other_'+tag+'@example.test';ot=register_email(other,pw)['token']
        call('GET','/api/v1/trips/'+trip_id,token=ot,want=404);call('GET','/api/v1/planning-jobs/'+j['id'],token=ot,want=404);call('POST','/api/v1/admin/places',{'id':places['items'][0]['id'],'duration_minutes':90,'active':True},token=ot,want=403);checks.append('ownership + admin role enforced')
        travel.terminate();travel.wait(timeout=15);launch('travel-restarted',['bin/travel']);time.sleep(2);assert call('GET','/api/v1/trips/'+trip_id,token=token)['version']==3;checks.append('process restart persistence')
        call('POST','/api/v1/auth/logout',token=token);call('GET','/api/v1/me',token=token,want=401);checks.append('logout revocation')
        report={'mode':'fixture_providers_real_http_rpc_postgresql','passed':len(checks),'checks':checks,'log_directory':str(logdir)};(logdir/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));print(json.dumps(report,ensure_ascii=False,indent=2))
    finally:
        for p in reversed(processes):
            if p.poll() is None:p.terminate()
        for p in reversed(processes):
            try:p.wait(timeout=15)
            except subprocess.TimeoutExpired:p.kill();p.wait()
        for f in logs:f.close()
        mailbox.close()
if __name__=='__main__':run()
