import type {CSSProperties} from "react";
import {formatBytes} from "../lib/format";
import {useStrings} from "../i18n";
import {IconMore,IconRefresh,IconShield} from "../ui/icons";
import {computeUserStatus,getUserQuota,isOnline} from "./users.helpers";
import {QuickConnectionLinks} from "./ConnectionLinks";
import {useUserRowGestures,type SwipeSide} from "./useUserRowGestures";
import type {UsersTopicQuotaEntry,UsersTopicUser} from "../realtime/topics";

export interface UserCardProps {
  user:UsersTopicUser; quotaEntry:UsersTopicQuotaEntry|undefined; now:number;
  gesturesEnabled?:boolean; swipeSide?:SwipeSide;
  canResetQuota?:boolean; canToggle?:boolean;
  onOpen:()=>void; onActions:(anchor?:DOMRect)=>void;
  onResetQuota:()=>void; onToggle:()=>void; onSwipeChange:(side:SwipeSide)=>void;
}

export function UserCard({user,quotaEntry,now,gesturesEnabled=false,swipeSide=null,canResetQuota=false,canToggle=false,onOpen,onActions,onResetQuota,onToggle,onSwipeChange}:UserCardProps) {
  const s=useStrings(),t=s.people.workspace;
  const quota=getUserQuota(user,quotaEntry);
  const exhausted=quota.limitBytes!==null&&quota.limitBytes>0&&quota.usedBytes!==null&&quota.usedBytes>=quota.limitBytes;
  const status=computeUserStatus(user,quota,now);
  const online=isOnline(user);
  const statusText=gesturesEnabled?(user.enabled?(online?s.people.online:s.people.offline):s.people.status.disabled):status==="active"?(online?s.people.online:s.people.offline):s.people.status[status];
  const {gestureProps,activationProps,onTap,offset,dragging}=useUserRowGestures({enabled:gesturesEnabled,side:swipeSide,onChange:onSwipeChange,onOpen,onMenu:()=>onActions(),description:t.gestureHelp});
  const used=quota.usedBytes===null?"—":formatBytes(quota.usedBytes,s);
  const limit=quota.limitBytes===null?t.unlimited:formatBytes(quota.limitBytes,s);
  const quotaText=quota.usedBytes===null?t.quotaUnknown:`${used} / ${limit}`;
  const style:CSSProperties={transform:gesturesEnabled?`translateX(${offset}px)`:undefined};
  return <div className="user-row-shell" data-testid="user-row" data-user={user.username} data-swipe={swipeSide??"closed"} data-quota-warning={exhausted} {...activationProps} onKeyDown={e=>{if(e.key==="ContextMenu"||(e.shiftKey&&e.key==="F10")){e.preventDefault();onActions();}}}>
    {gesturesEnabled&&<>
      <div className="user-side user-side-quota" aria-hidden={swipeSide!=="left"}><span>{t.usedQuota}</span><strong>{quotaText}</strong><button type="button" aria-label={`${t.resetQuota} ${user.username}`} tabIndex={swipeSide==="left"?0:-1} disabled={!canResetQuota||quota.usedBytes===null} onClick={onResetQuota}><IconRefresh/>{t.resetQuota}</button></div>
      <button type="button" className={`user-side user-side-access ${user.enabled?"":"enable"}`} aria-label={`${user.enabled?s.people.actions.disable:s.people.actions.enable} ${user.username}`} aria-hidden={swipeSide!=="right"} tabIndex={swipeSide==="right"?0:-1} disabled={!canToggle} onClick={onToggle}><IconShield/><strong>{user.enabled?s.people.actions.disable:s.people.actions.enable}</strong></button>
    </>}
    <div className="user-row-face" {...gestureProps} style={style} data-dragging={dragging} onContextMenu={e=>gesturesEnabled&&e.preventDefault()}>
      <button type="button" className="user-identity" data-testid={`user-card-${user.username}`} aria-label={`${t.openPerson} ${user.username}`} aria-description={exhausted?t.exhausted:undefined} onClick={onTap}><strong title={user.username}>{user.username}</strong><span className={exhausted?"text-warn":online?"text-ok":"text-text-muted"}>{statusText}{gesturesEnabled&&user.enabled&&<span> · {user.current_connections} / {user.active_unique_ips} IP</span>}</span></button>
      <div className="user-col-now"><strong>{user.current_connections}</strong><small>{user.active_unique_ips} {s.people.activeIps}</small></div>
      <div className="user-col-traffic"><strong>{user.traffic?formatBytes(user.traffic.observed_total_bytes,s):"—"}</strong><small>{quotaText}</small></div>
      <div className="user-col-expiry"><strong title={user.expiration_rfc3339}>{user.expiration_rfc3339?.slice(0,10)??s.people.detail.noExpiry}</strong></div>
      <div className="user-row-actions" inert={gesturesEnabled&&swipeSide!==null}><QuickConnectionLinks user={user}/>{!gesturesEnabled&&<button type="button" className="user-menu-trigger" aria-label={`${s.people.actions.menu} ${user.username}`} onClick={e=>onActions(e.currentTarget.getBoundingClientRect())}><IconMore/></button>}</div>
    </div>
  </div>;
}
