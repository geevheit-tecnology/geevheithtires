import {readFile,writeFile} from 'node:fs/promises';
const input=new URL('../api/internal/commercial/catalog.json',import.meta.url);
const output=new URL('../web/lib/plan-catalog.generated.json',import.meta.url);
const source=await readFile(input,'utf8');
if(process.argv.includes('--check')) {
 if(await readFile(output,'utf8')!==source)throw Error('Catálogo de planos desatualizado. Execute node scripts/sync-plan-catalog.mjs.');
 console.log('Catálogo de planos sincronizado.');
} else {await writeFile(output,source);}
