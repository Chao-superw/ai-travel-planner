#!/usr/bin/env python3
"""Keep domain wire models and Thrift structs in sync. Run before kitex generation."""
import pathlib, re
root = pathlib.Path(__file__).resolve().parents[1]
src = (root/'internal/domain/types.go').read_text()
models = re.findall(r'type (\w+) struct \{(.*?)\n\}',src,re.S)
names = {name for name,_ in models}
def thrift_type(t):
    t=t.strip().removeprefix('*')
    if t.startswith('[]'): return 'list<'+thrift_type(t[2:])+'>'
    if t=='map[string]string': return 'map<string,string>'
    return {'int32':'i32','int64':'i64','bool':'bool','string':'string'}.get(t,t)
existing={}
model_path=root/'idl/model.thrift'
if model_path.exists():
    for name,body in re.findall(r'struct (\w+)\s*\{(.*?)\}',model_path.read_text(),re.S):
        existing[name]={key:(int(number),typ,bool(optional)) for number,optional,typ,key in re.findall(r'(\d+):\s*(?:(optional|required)\s+)?(\S+)\s+(\w+)',body)}
lines=['// Generated from internal/domain/types.go by scripts/generate_idl.py','namespace go model','']
for name,body in models:
    lines.append('struct '+name+' {')
    fields=re.findall(r'\s+(\w+)\s+([^`]+)`json:"([^",]+)(?:,[^"]*)?"`',body)
    old=existing.get(name,{})
    current={key for _,_,key in fields}
    if old.keys()-current: raise ValueError('removing existing wire fields requires explicit migration: '+name)
    next_id=max((entry[0] for entry in old.values()),default=0)+1
    for _,typ,key in fields:
        wire_type=thrift_type(typ); is_optional=typ.strip().startswith('*')
        if key in old:
            n,old_type,old_optional=old[key]
            if wire_type!=old_type or is_optional!=old_optional:
                raise ValueError('incompatible wire field change: '+name+'.'+key)
        else:
            n=next_id;next_id+=1
        optional='optional ' if is_optional else ''
        lines.append(f'  {n}: {optional}{wire_type} {key}')
    lines.extend(['}',''])
(root/'idl/model.thrift').write_text('\n'.join(lines))
travel=['Register','Login','Logout','GetCurrentUser','SearchPlaces','CreatePlace','UpdatePlace','CreatePlanningJob','GetPlanningJob','RetryPlanningJob','ListTrips','GetTrip','UpdateTrip','CreateReplanningJob','ListTripVersions','GetTripVersion','GetBudgetSummary','Health','RequestRegistrationCode','LegacyLogin','RequestEmailBindingCode','BindEmail']
for name,methods,req,resp in [('travel',travel,'Request','Response'),('planner',['GenerateItinerary','ReviseItinerary'],'PlanningRequest','PlanningResult')]:
    body='namespace go '+name+'\ninclude "model.thrift"\nservice '+name.title()+'Service {\n'
    body+='\n'.join(f'  model.{resp} {m}(1: model.{req} req)' for m in methods)+'\n}\n'
    (root/f'idl/{name}.thrift').write_text(body)
