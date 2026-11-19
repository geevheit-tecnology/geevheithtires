import catalog from "./plan-catalog.generated.json";
export const planCatalog = catalog.plans;
export const featureCatalog: Record<string, {name:string;benefit:string;available:boolean}> = catalog.features;
export const provisionalPrice = "R$ 0,00";
