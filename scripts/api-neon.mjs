import {spawn} from "node:child_process";
const mode=process.argv[2]||"server";
if(!process.env.DATABASE_URL_UNPOOLED)throw new Error("Execute neon deploy para obter DATABASE_URL_UNPOOLED.");
const env={...process.env,DATABASE_URL:process.env.DATABASE_URL_UNPOOLED,PORT:process.env.PORT||"8097",APP_URL:process.env.APP_URL||"http://localhost:3017"};
if(mode==='test')env.TEST_DATABASE_URL=env.DATABASE_URL;
const args=mode==='test'?['test','-v','./...']:['run',`./cmd/${mode==='migrate'?'migrate':'server'}`];
const child=spawn('go',args,{cwd:new URL('../api/',import.meta.url),env,stdio:'inherit'});
for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>child.kill(signal));
child.on('exit',code=>process.exit(code??1));
