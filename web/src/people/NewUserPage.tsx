import {Link,useNavigate} from "@tanstack/react-router";
import {useContext,useState} from "react";
import {useStrings} from "../i18n";
import {PeopleContext} from "./PeopleContext";
import {UserFormSheet} from "./UserFormSheet";
import {useUserFormBlocker} from "./useUserFormBlocker";

export function NewUserPage() {
  const s=useStrings(),access=useContext(PeopleContext),navigate=useNavigate();
  const [created,setCreated]=useState<string|null>(null);
  const {setDirty,confirmation}=useUserFormBlocker();
  return <div className="user-detail-page"><Link className="user-back" to="/people">← {s.people.workspace.back}</Link><h1>{s.people.workspace.newUser}</h1>
    <UserFormSheet inline open mode="create" disabled={access.readOnly} onDirtyChange={setDirty} onSaved={username=>{setCreated(username);setDirty(false);}} onClose={()=>{if(created)void navigate({to:"/people/$username",params:{username:created}});else void navigate({to:"/people"});}} onConfigureWeb={username=>void navigate({to:"/people/$username",params:{username},search:{tab:"access"}})}/>
    {confirmation}
  </div>;
}
