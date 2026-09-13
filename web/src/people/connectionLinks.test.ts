import {describe,it,expect} from "vitest";
import {collectConnectionLinks,formatConnectionLink,webProfileIndex} from "./connectionLinks";
import type {UserLinksWire} from "../realtime/topics";
const secret="0123456789abcdef0123456789abcdef";
const hex=(value:string)=>[...new TextEncoder().encode(value)].map(b=>b.toString(16).padStart(2,"0")).join("");
const link=(prefix="",host="proxy.example.org",domain="")=>`tg://proxy?${new URLSearchParams({server:host,port:"443",secret:prefix+secret+(domain?hex(domain):"")})}`;
const links=(override:Partial<UserLinksWire>={}):UserLinksWire=>({classic:[],secure:[],tls:[],tls_domains:[],...override});

describe("connection link projection",()=>{
  it("keeps primary TLS, extra domains and multiple endpoints without duplication",()=>{
    const primary=link("ee","proxy.example.org","main.example.org"),extra=link("ee","proxy.example.org","extra.example.org"),other=link("ee","198.51.100.4","main.example.org");
    const result=collectConnectionLinks(links({tls:[primary,extra,other],tls_domains:[{domain:"extra.example.org",link:extra}]}));
    expect(result).toHaveLength(3);expect(result.filter(l=>l.primary)).toHaveLength(1);expect(result[0]?.domain).toBe("main.example.org");expect(result[2]?.endpoint).toBe("198.51.100.4:443");
  });
  it("keeps all secure/classic entries",()=>{
    expect(collectConnectionLinks(links({secure:[link("dd"),link("dd","198.51.100.2")],classic:[link(),link("","198.51.100.3")]}))).toHaveLength(4);
  });
  it("does not replace a malformed primary with another variant",()=>{
    const result=collectConnectionLinks(links({tls:["invalid",link("ee","p.example","mask.example")]}));expect(result).toHaveLength(1);expect(result[0]?.primary).toBe(false);
  });
  it("preserves query fields in the t.me representation",()=>{
    const l=collectConnectionLinks(links({secure:[link("dd")+"&comment=alice%20phone"]}))[0]!;
    const tg=new URL(l.url),tme=new URL(formatConnectionLink(l,"tme"));expect(tme.origin).toBe("https://t.me");expect(tme.search).toBe(tg.search);
  });
  it("derives WEB from a validated TLS-only secret and preserves profile order",()=>{
    const result=collectConnectionLinks(links({tls:[link("ee","p.example","mask.example")]}),[{host:"web.example",mode:"plain"},{host:"web.example",mode:"dd"}]).filter(l=>l.kind==="web");
    expect(result.map(l=>l.profileMode)).toEqual(["plain","dd"]);expect(result.map(l=>l.primary)).toEqual([true,false]);
    const u=new URL(result[0]!.url);expect(u.hostname).toBe("webproxy");expect(u.searchParams.get("secret")).toBe(secret);expect(u.searchParams.has("port")).toBe(false);expect(u.searchParams.has("comment")).toBe(false);
    expect(formatConnectionLink(result[0]!,"tme")).toBe(result[0]!.url);
  });
  it("does not invent a WEB secret or generate a link for conflicting secrets",()=>{
    const profiles=[{host:"web.example",mode:"dd" as const}];
    expect(collectConnectionLinks(links(),profiles)).toEqual([]);
    const conflict=link("dd").replace(secret,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
    expect(collectConnectionLinks(links({classic:[link()],secure:[conflict]}),profiles).some(l=>l.kind==="web")).toBe(false);
  });
  it.each(["javascript:alert(1)","tg://resolve?domain=example",`https://evil.example/proxy?server=h&port=443&secret=${secret}`,link()+"&secret="+secret,link().replace("port=443","port=0"),link().replace("port=443","port=65536"),link("ee","p.example","mask.example").slice(0,-1)])("rejects invalid or unrelated proxy URLs: %s",raw=>{
    expect(collectConnectionLinks(links({classic:[raw],tls:[raw]}))).toEqual([]);
  });
  it("indexes only enabled WEB and keeps each account's own profiles",()=>{
    const view={revision:"r",enabled:true,vhosts:[{host:"a.example",public_addr:"198.51.100.1:443",profiles:[{user:"alice",secret_mode:"plain" as const},{user:"bob",secret_mode:"dd" as const}]},{host:"b.example",public_addr:"198.51.100.2:443",profiles:[{user:"alice",secret_mode:"dd" as const}]}]};
    expect(webProfileIndex(view).get("alice")).toEqual([{host:"a.example",mode:"plain"},{host:"b.example",mode:"dd"}]);expect(webProfileIndex({...view,enabled:false}).size).toBe(0);
  });
});
