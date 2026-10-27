"use client";
import {createContext, useContext, ReactNode} from "react";
import {featureCatalog} from "../lib/plan-catalog";
export type PlanAccess={plan_id:string;plan_name:string;status:string;can_write:boolean;vehicle_limit:number;user_limit:number;features:Record<string,boolean>};
export const lockedAccess:PlanAccess={plan_id:"starter",plan_name:"Starter",status:"loading",can_write:false,vehicle_limit:15,user_limit:3,features:{}};
const PlanContext=createContext({access:lockedAccess,viewPlans:()=>{},refresh:async()=>{}});
export function PlanAccessProvider({access,viewPlans,refresh,children}:{access:PlanAccess;viewPlans:()=>void;refresh:()=>Promise<void>;children:ReactNode}) {
 return <PlanContext.Provider value={{access,viewPlans,refresh}}>{children}</PlanContext.Provider>;
}
export const usePlanAccess=()=>useContext(PlanContext);
export function UpgradeNotice({feature}:{feature:string}) {
 const {viewPlans}=usePlanAccess();const item=featureCatalog[feature];
 return <aside className="plan-upgrade"><span className="plan-level">EXCLUSIVO PRO E ENTERPRISE</span><h3>{item?.name||"Recursos avançados"}</h3><p>{item?.benefit}</p><button type="button" className="table-link" onClick={viewPlans}>Conhecer os benefícios dos planos →</button></aside>;
}
export function PlanBenefits({features,planned=[]}:{features:string[];planned?:string[]}) {
 return <><ul className="plan-benefits">{features.map(key=><li key={key}><strong>{featureCatalog[key]?.name}</strong><span>{featureCatalog[key]?.benefit}</span></li>)}</ul>{planned.length>0&&<div className="planned-benefits"><strong>Previstos, ainda indisponíveis</strong><p>{planned.map(key=>featureCatalog[key]?.name).join(" · ")}</p></div>}</>;
}
