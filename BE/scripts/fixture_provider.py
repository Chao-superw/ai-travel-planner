#!/usr/bin/env python3
"""Explicit local-only provider fixture. Never used as a production fallback."""
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit, parse_qs
from datetime import date, timedelta
class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_): pass
    def send(self, data, status=200):
        body=json.dumps(data,ensure_ascii=False).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(body)
    def do_GET(self):
        path=urlsplit(self.path);q=parse_qs(path.query)
        if path.path=='/v5/place/text':
            museum=q.get('keywords',[''])[0]=='博物馆';start=21 if museum else 1
            pois=[{'id':f'FIX{i:03d}','name':f'测试博物馆{i}' if museum else f'测试景点{i}','cityname':'杭州市','citycode':'0571','adcode':'330106','location':f'{120.1+i*.001:.6f},30.250000','typecode':'140100' if museum else '110200','address':'测试地点'} for i in range(start,start+20)]
            self.send({'status':'1','info':'OK','infocode':'10000','count':str(len(pois)),'pois':pois})
        elif path.path.endswith('/walking'):
            self.send({'status':'1','info':'OK','infocode':'10000','count':'1','route':{'paths':[{'distance':'800','cost':{'duration':'600'},'steps':[{'instruction':'测试路线','polyline':'120.101,30.25;120.102,30.25'}]}]}})
        elif path.path.endswith('/transit/integrated'):
            self.send({'status':'1','info':'OK','infocode':'10000','count':'1','route':{'transits':[{'distance':'1800','walking_distance':'100','cost':{'duration':'900','transit_fee':'2.0'},'segments':[{'bus':{'buslines':[{'name':'测试公交','polyline':'120.101,30.25;120.102,30.25'}]}}]}]}})
        else:self.send({'error':'not found'},404)
    def do_POST(self):
        if self.path!='/api/paas/v4/chat/completions':return self.send({'error':'not found'},404)
        body=json.loads(self.rfile.read(int(self.headers['Content-Length'])));r=json.loads(body['messages'][1]['content']);c=r['constraints'];places=r['places'];activities=[]
        if r.get('scope'):
            scope=r['scope'];used={a.get('place_id') for a in r['base']['activities']};chosen=next(p for p in places if p['id'] not in used)
            activities=[{'id':'replacement','date':scope['date'],'start_minute':scope['start_minute'],'end_minute':scope['start_minute']+90,'kind':'sightseeing','place_id':chosen['id'],'title':'室内替换景点','reason':'根据修改要求重新安排'}]
        else:
            d=date.fromisoformat(c['start_date']);end=date.fromisoformat(c['end_date']);i=0
            while d<=end:
                activities.extend([{'id':f'a{i}','date':str(d),'start_minute':540,'end_minute':630,'kind':'sightseeing','place_id':places[i]['id'],'title':'上午游览','reason':'偏好匹配'},{'id':f'm{i}','date':str(d),'start_minute':720,'end_minute':780,'kind':'meal','place_id':'','title':'午餐','reason':'预留休息时间'},{'id':f'b{i}','date':str(d),'start_minute':840,'end_minute':930,'kind':'sightseeing','place_id':places[i+1]['id'],'title':'下午游览','reason':'预留交通间隔'}])
                if d<end:activities.append({'id':f'h{i}','date':str(d),'start_minute':1200,'end_minute':1260,'kind':'hotel','place_id':'','title':'住宿安排','reason':'住宿地点待选'})
                d+=timedelta(days=1);i+=2
        out={'outcome':'candidate','plan':{'title':'测试行程','summary':'受控供应商测试数据','activities':activities,'warnings':[]},'issues':[]}
        self.send({'id':'fixture-model-response','model':body['model'],'choices':[{'finish_reason':'stop','message':{'content':json.dumps(out,ensure_ascii=False)}}],'usage':{'prompt_tokens':100,'completion_tokens':100}})
if __name__=='__main__':
    import argparse
    p=argparse.ArgumentParser();p.add_argument('--port',type=int,default=18081);args=p.parse_args();ThreadingHTTPServer(('127.0.0.1',args.port),Handler).serve_forever()
