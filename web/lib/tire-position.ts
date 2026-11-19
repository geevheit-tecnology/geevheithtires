export function positionLabel(position:string, positions?:string[]) {
 if(position.startsWith("STEP")) return `Estepe ${position.slice(4)}`;
 const match=/^(\d+)([ED])([EI])$/.exec(position);
 if(!match) return position;
 const [,axle,side,wheel]=match;
 const index=positions?.indexOf(position);
 return `${index!==undefined&&index>=0?`Posição ${index+1} · `:""}Eixo ${axle} · lado ${side==="E"?"esquerdo":"direito"} · ${wheel==="I"?"interno":"externo"} (${position})`;
}
