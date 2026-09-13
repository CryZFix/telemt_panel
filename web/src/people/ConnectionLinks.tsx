import {useContext, useMemo, useState} from "react";
import {PeopleContext} from "./PeopleContext";
import {collectConnectionLinks, formatConnectionLink, type ConnectionKind, type ConnectionLink, type WebLinkProfile} from "./connectionLinks";
import type {UsersTopicUser} from "../realtime/topics";
import {useStrings} from "../i18n";
import {copyText} from "../lib/copyText";
import {pushToast} from "../ui/Toast";
import {Sheet} from "../ui/Sheet";
import {CopyField} from "../ui/CopyField";
import {QR} from "../ui/QR";
import {Select} from "../ui/Select";
import {Button} from "../ui/Button";

const emptyProfiles: WebLinkProfile[] = [];
function useLinks(user: UsersTopicUser) {
  const context = useContext(PeopleContext);
  const profiles = context.profiles.get(user.username) ?? emptyProfiles;
  const links = useMemo(()=>collectConnectionLinks(user.links, profiles),[user.links,profiles]);
  return {...context,links,profiles,format:context.formats.get(user.username)??"tg" as const};
}
const names: Record<ConnectionKind,string> = {tls:"EE",secure:"DD",classic:"Classic",web:"WEB"};

export function QuickConnectionLinks({user}: {user: UsersTopicUser}) {
  const s=useStrings(), t=s.people.workspace;
  const {links,profiles,format,setFormat}=useLinks(user);
  const [manual,setManual]=useState<string|null>(null);
  const kinds: ConnectionKind[] = (["tls","secure","web"] as const).filter(kind=>links.some(l=>l.kind===kind&&l.primary)||(kind==="web"&&profiles.length>0));
  async function copy(link:ConnectionLink){const text=formatConnectionLink(link,format);if(await copyText(text)==="failed")setManual(text);else pushToast(`${user.username} · ${names[link.kind]} · ${s.common.copied}`,"ok");}
  if(!kinds.length)return null;
  return <><div className="user-copy-toolbar" role="group" aria-label={`${t.quickLinks} ${user.username}`}>
    <button type="button" className="user-format" aria-label={`${t.format} ${user.username}: ${format==="tg"?"tg://":"t.me"}`} title={t.formatHint} onClick={()=>setFormat(user.username,format==="tg"?"tme":"tg")}>{format==="tg"?"tg://":"t.me"}<span aria-hidden="true">⇄</span></button>
    {kinds.map(kind=>{const link=links.find(l=>l.kind===kind&&l.primary);return <button type="button" key={kind} className="user-copy-kind" disabled={!link} title={!link?t.webSecretUnavailable:kind==="web"?t.webFormat:t.primaryLink} aria-label={`${s.common.copy} ${names[kind]} · ${user.username}`} onClick={()=>link&&void copy(link)}>{names[kind]}</button>;})}
  </div><Sheet open={manual!==null} onClose={()=>setManual(null)} title={s.common.copyManually}><CopyField value={manual??""}/></Sheet></>;
}

export function ConnectionLinks({user}: {user:UsersTopicUser}) {
  const s=useStrings(),t=s.people.workspace;
  const {links,profiles,webUnavailable,setFormat}=useLinks(user);
  const [kind,setKind]=useState<ConnectionKind>(links[0]?.kind??"tls");
  const [selected,setSelected]=useState("");
  const [qr,setQR]=useState(false);
  const kinds=(["tls","secure","classic","web"] as const).filter(k=>links.some(l=>l.kind===k));
  const activeKind=kinds.includes(kind)?kind:kinds[0];
  const options=links.filter(l=>l.kind===activeKind);
  const link=options.find(l=>l.url===selected)??options[0];
  const [manual,setManual]=useState<string|null>(null);
  async function copy(format:"tg"|"tme") {if(!link)return;const value=formatConnectionLink(link,format);if(link.kind!=="web")setFormat(user.username,format);if(await copyText(value)==="failed")setManual(value);else pushToast(s.common.copied,"ok");}
  return <section className="user-section"><h2>{s.people.detail.linksTitle}</h2>
    {webUnavailable&&<p className="user-note">{s.people.webAccess.unavailable}</p>}
    {!!profiles.length&&!links.some(l=>l.kind==="web")&&<p className="user-note text-warn">{t.webSecretUnavailable}</p>}
    {!link?<p className="user-note">{s.people.detail.noLinks}</p>:<>
      <div className="user-link-modes" role="group" aria-label={t.linkKind}>{kinds.map(k=><button type="button" key={k} aria-pressed={activeKind===k} onClick={()=>{setKind(k);setSelected("");setManual(null);setQR(false);}}>{names[k]}</button>)}</div>
      <label className="user-field"><span>{t.linkVariant}</span><Select value={link.url} onChange={e=>{setSelected(e.target.value);setManual(null);setQR(false);}}>{options.map(l=><option key={l.url} value={l.url}>{[l.endpoint,l.domain,l.profileMode,l.primary?t.primary:null].filter(Boolean).join(" · ")}</option>)}</Select></label>
      <div className="user-buttons"><Button onClick={()=>void copy("tg")}>{s.common.copy} {activeKind==="web"?"WEB":"tg://"}</Button>{activeKind!=="web"&&<Button variant="secondary" onClick={()=>void copy("tme")}>{s.common.copy} t.me</Button>}<Button variant="secondary" onClick={()=>setQR(!qr)}>QR</Button></div>
      <p className="user-note">{activeKind==="web"?t.webFormat:t.primaryHint}</p>
      <details className="user-link-details"><summary>{t.showLink}</summary><CopyField value={link.url}/></details>
      {qr&&<QR value={link.url} size={160}/>}
      {manual&&<><p className="user-note">{s.common.copyManually}</p><CopyField value={manual}/></>}
    </>}
  </section>;
}
