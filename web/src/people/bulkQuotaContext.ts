import {createContext,useContext} from "react";

export const BulkQuotaContext=createContext({running:false,openMenu:(_anchor?:DOMRect)=>{}});
export function useBulkQuota(){return useContext(BulkQuotaContext);}
