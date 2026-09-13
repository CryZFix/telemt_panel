import {useCallback,useRef,useState} from "react";
import {useBlocker} from "@tanstack/react-router";
import {useStrings} from "../i18n";
import {Sheet} from "../ui/Sheet";
import {Button} from "../ui/Button";

export function useUserFormBlocker() {
  const s=useStrings();
  const dirty=useRef(false);
  const [hasChanges,setHasChanges]=useState(false);
  const setDirty=useCallback((value:boolean)=>{dirty.current=value;setHasChanges(value);},[]);
  const blocker=useBlocker({shouldBlockFn:()=>dirty.current,enableBeforeUnload:hasChanges,withResolver:true});
  const confirmation=<Sheet open={blocker.status==="blocked"} onClose={()=>blocker.reset?.()} title={s.people.workspace.discard}>
    <p className="user-note">{s.people.workspace.discardNote}</p><div className="user-buttons"><Button variant="secondary" onClick={()=>blocker.reset?.()}>{s.people.workspace.stay}</Button><Button variant="danger" onClick={()=>{setDirty(false);blocker.proceed?.();}}>{s.people.workspace.leave}</Button></div>
  </Sheet>;
  return {setDirty,hasChanges,confirmation};
}
