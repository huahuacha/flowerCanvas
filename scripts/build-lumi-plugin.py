import json,zipfile,copy
from pathlib import Path
root=Path(__file__).resolve().parents[1]
ref=lambda p:{'$ref':p}
opt=lambda k:ref('request.providerOptions.lumi-ark-video.'+k)
op=lambda k,*v:{'$'+k:list(v)}
eq=lambda a,b:op('eq',a,b)
ne=lambda a,b:op('ne',a,b)
both=lambda *a:op('and',*a)
either=lambda *a:op('or',*a)
when=lambda c,a,b=None:{'$if':{'condition':c,'then':a,'else':b}}
nonempty=lambda a:op('gt',{'$len':a},0)
upgrade=nonempty(opt('draft_task_id'))
normal={'$not':upgrade}
model25=op('in',ref('request.model'),['seedance-2.5','doubao-seedance-2-5-260628'])
value=lambda key,fallback=None:{'$coalesce':[opt(key),fallback]} if fallback is not None else opt(key)
duration=value('duration',when(ne(ref('request.duration'),0),ref('request.duration')))
ratio=value('ratio',{'$omitEmpty':ref('request.aspectRatio')})
resolution=value('resolution',{'$omitEmpty':ref('request.resolution')})
content=[when(nonempty(ref('request.prompt')),[{'type':'text','text':ref('request.prompt')}],[])]
for plural,kind,role in [('images','image_url','reference_image'),('videos','video_url','reference_video'),('audios','audio_url','reference_audio')]:
 content.append({'$map':{'from':{'$sortByOrder':ref('request.'+plural)},'as':'media','in':{'type':kind,kind:{'url':ref('media.value')},'role':{'$coalesce':[ref('media.role'),role]}}}})
body={'model':ref('request.model'),'content':when(upgrade,[{'type':'draft_task','draft_task':{'id':opt('draft_task_id')}}],{'$concatArrays':content}), 'ratio':when(normal,ratio),'resolution':resolution,'duration':when(normal,duration)}
fields={'generate_audio':'boolean','watermark':'boolean','return_last_frame':'boolean','is_flf':'boolean','output_format':'string','tools':'array','priority':'integer','draft':'boolean','camera_fixed':'boolean','seed':'integer','execution_expires_after':'integer','omni_reference_task_type':'string','project_name':'string','video_name':'string','metadata':'object','safety_identifier':'string'}
for k in fields:
 fallback=None
 if k=='generate_audio': fallback=when(eq(ref('request.generateAudio'),True),True)
 if k=='watermark': fallback=when(eq(ref('request.watermark'),True),True)
 body[k]=when(normal,value(k,fallback)) if k in ['generate_audio','return_last_frame','is_flf','tools','camera_fixed','seed','omni_reference_task_type'] else opt(k)
rules=[]
def rule(c,m):rules.append({'assert':c,'message':m})
rule(eq(ref('request.extra.lumiConfigError'),None),'Lumi 参数不是合法 JSON 对象')
rule(nonempty(ref('request.model')),'请选择 Lumi 模型')
rule(nonempty(ref('request.extra.idempotencyKey')),'Lumi 提交缺少稳定幂等键')
rule(either(upgrade,nonempty(ref('request.prompt')),nonempty(ref('request.inputs')),nonempty(ref('request.images')),nonempty(ref('request.videos')),nonempty(ref('request.audios'))),'请填写提示词或添加参考素材')
rule(either(model25,eq({'$len':ref('request.audios')},0),nonempty(ref('request.images')),nonempty(ref('request.videos'))),'仅 Seedance 2.5 支持纯音频参考')
rule(either(eq(duration,None),eq(duration,-1),both(op('gte',duration,4),op('lte',duration,when(model25,30,15)))),'视频时长必须为 4～15 秒（2.5 为 4～30 秒）或 -1')
rule(either(eq(opt('priority'),None),both(op('gte',opt('priority'),0),op('lte',opt('priority'),9))),'优先级须为 0～9')
for plural,n in [('images',9),('videos',3),('audios',3)]:rule(op('lte',{'$len':ref('request.'+plural)},n),'参考素材数量超出协议限制')
rule(either({'$not':upgrade},both(model25,eq(resolution,'1080p'))),'样片升清仅支持 Seedance 2.5，分辨率必须为 1080p')
rule(either(ne(opt('draft'),True),both(model25,either(eq(resolution,None),eq(resolution,'480p')))),'样片仅支持 Seedance 2.5 的 480p 输出')
for k in ['callback_url','frames']:rule(eq(opt(k),None),k+' 当前不开放')
rule(op('in',value('service_tier','default'),['default']),'仅支持默认服务等级')
edit=op('in',opt('omni_reference_task_type'),['edit','extend'])
rule(either({'$not':edit},both(model25,nonempty(ref('request.videos')),eq(ratio,'adaptive'))),'编辑或续写需要 Seedance 2.5、视频参考和 adaptive 画幅')
rule(either(ne(opt('omni_reference_task_type'),'edit'),eq(duration,-1)),'编辑模式时长必须为 -1')
rules.append({'assert':either(normal,both(eq({'$len':ref('request.images')},0),eq({'$len':ref('request.videos')},0),eq({'$len':ref('request.audios')},0))), 'message':'升清仅提交原平台样片任务 ID，不可同时附加素材'})
params=[{'name':k,'type':t,'mapping':k,'description':'Lumi 平台参数 '+k} for k,t in {'model':'string','prompt':'string','images':'media[]','videos':'media[]','audios':'media[]','duration':'integer','ratio':'string','resolution':'string','draft_task_id':'string',**fields}.items()]
video={'$coalesce':[ref('response.relay_video_url'),ref('response.content.video_url')]}
success=both(eq(ref('response.status'),'succeeded'),nonempty(video))
status=when(eq(ref('response.reconciliation_pending'),True),'processing',when(op('in',ref('response.status'),['queued','running','succeeded']),when(success,'succeeded','processing'),'failed'))
provider={'id':'lumi-ark-video','label':'Lumi 视频（火山兼容）','capabilities':['video'],'scopes':['admin.system-channel','user.custom-channel','canvas','creation','agent'],'baseUrl':'https://oasisath.cn','requiresPublicMediaUrls':True,'idempotentCreate':True,'detachedPolling':True,'auth':{'type':'bearer','field':'apiKey'},'parameters':params,'validations':rules,'create':{'method':'POST','path':'/api/v3/contents/generations/tasks','contentType':'application/json','headers':{'Idempotency-Key':ref('request.extra.idempotencyKey')},'body':body},'poll':{'method':'GET','path':'/api/v3/contents/generations/tasks/{{taskId}}'},'response':{'taskId':{'$coalesce':[ref('response.id'),ref('taskId')]},'status':status,'message':{'$coalesce':[ref('response.error.message'),'Lumi 任务未完成或返回未知状态，请查询原任务']},'videos':when(success,video),'images':when(success,{'$coalesce':[ref('response.relay_last_frame_url'),ref('response.content.last_frame_url')]}),'usage':{'$merge':[ref('response.usage'),{'consumed_tokens':ref('response.consumed_tokens'),'consumed_tokens_source':ref('response.consumed_tokens_source'),'billing_status':ref('response.billing_status'),'consumed_amount':ref('response.consumed_amount'),'currency':ref('response.currency'),'duration':ref('response.duration'),'framespersecond':ref('response.framespersecond')}]},'errorPaths':['error.code'],'resultEphemeral':True}}
manifest={'apiVersion':'yingce.plugin/v2','id':'lumi-ark-video','name':'Lumi 视频（火山兼容）','version':'1.0.0','author':'Local integration','description':'通过 New API 平台调用 Lumi Seedance；独立协议，不改变原火山适配。','permissions':['generation.run','media.read'],'configuration':{'fields':[{'name':'apiKey','type':'secret','label':'平台 API Key','required':True}]},'contributes':{'providers':[provider]}}
p=root/'plugin-packages/lumi-ark-video';(p/'docs').mkdir(parents=True,exist_ok=True)
readme='# Lumi 视频（火山兼容）\n\n连接你的 New API 根地址，使用平台 apiKey。当前应用开放普通视频、参考素材及 Seedance 2.5 样片模式。升清/编辑/续写模板已预留，但应用准入暂不开放，避免规格计费不一致。界面中的升清 ID 必须是当前平台自己的成功样片 task_ ID。\n\n此包独立、可搬迁，不依赖 DreamCanvas 或本机绝对路径。通过影策既有插件中心安装或随 plugin-packages 分发。取消与回调未开放。\n'
docs='# 接口与参数\n\n创建 POST /api/v3/contents/generations/tasks；查询 GET 同路径/{平台任务ID}。每笔新创作使用宿主生成的稳定 Idempotency-Key，重试沿用，已有任务不重新提交。\n\n应用边界：当前要求非空提示词和明确时长、画幅、分辨率；不开放自动时长、升清、编辑和续写。尾帧链接可由平台查询，当前影策视频结果尚未持久化为独立尾帧资产。409 submission_pending 不自动重建任务，留待人工恢复或后续状态恢复流程。\n\n参数来源为 providerOptions.lumi-ark-video；画布视频设置中的 Lumi 面板写入 lumiVideoParameters，宿主转换后按本清单映射。参数显式覆盖普通面板；未设置的参数不补默认值。\n\n'+ '\n'.join('- `'+x['name']+'`：'+x['description'] for x in params)+'\n\n响应：保留平台任务 ID，优先 relay_video_url/relay_last_frame_url，缺失回退 content 原始链接；临时资源仍由宿主持久化。生成结束与费用结算分开，usage/consumed_tokens 不用于再次计费。账务信息来自平台，不在此换算 Lumi 算点。成功但无视频、reconciliation_pending 保持查询原任务。未知状态按失败处理。\n\n参考素材必须是 HTTPS 或平台允许的资产引用；不要填本地路径、HTTP、Base64。asset ID 是 New API 平台资产，不是影策本地素材 ID。\n\n样片升清仅传 draft_task_id 与 1080p，普通时长/声音等字段省略。平台负责样片归属和上游账号绑定。修改模型后请检查可用能力。真实上游生成与升清对账尚未验证。\n'
placeholder='<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>'
contract=copy.deepcopy(manifest);contract['documentation']=placeholder
docs+='\n<!-- YINGCE_MANIFEST_CONTRACT_START -->\n## Manifest 完整接口定义\n\n```json\n'+json.dumps(contract,ensure_ascii=False,indent=2)+'\n```\n<!-- YINGCE_MANIFEST_CONTRACT_END -->\n'
manifest['documentation']=readme.strip()+'\n\n---\n\n'+docs.strip()
(p/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
(p/'README.md').write_bytes(readme.encode('utf-8'));(p/'docs/interface.md').write_bytes(docs.encode('utf-8'))
with zipfile.ZipFile(p.with_suffix('.yingce-plugin'),'w',zipfile.ZIP_DEFLATED) as z:
 for f in sorted(p.rglob('*')):
  if f.is_file():z.write(f,f.relative_to(p).as_posix())
print('Lumi plugin built')
