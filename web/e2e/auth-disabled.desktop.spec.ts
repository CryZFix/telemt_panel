import {test,expect} from "./fixtures";
import {spawn,execFileSync,type ChildProcess} from "node:child_process";
import {mkdtemp,writeFile,rm} from "node:fs/promises";
import {tmpdir} from "node:os";
import path from "node:path";
import {fileURLToPath} from "node:url";
import {MOCK_URL,ADMIN_PASSWORD,ADMIN_USERNAME} from "./env";

test("config-only anonymous access, protected auth operations and re-enable",async({page},testInfo)=>{
  test.setTimeout(60000);
  const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"../..");
  const binary=process.env["TELEMT_PANEL_E2E_BINARY"]??path.join(root,"telemt-panel");
  const scratch=await mkdtemp(path.join(tmpdir(),"panel-auth-mode-"));
  const configFile=path.join(scratch,"panel.toml");
  const origin="http://localhost:48184",base=origin+"/panel";
  const hash=execFileSync(binary,["hash-password"],{input:ADMIN_PASSWORD+"\n",encoding:"utf8"}).trim();
  let panelProcess:ChildProcess|undefined;
  let logs="";
  async function stop(){
    if(!panelProcess||panelProcess.exitCode!==null||panelProcess.signalCode!==null)return;
    const child=panelProcess;await new Promise<void>(resolve=>{child.once("exit",()=>resolve());child.kill("SIGTERM");});
  }
  async function start(disabled:boolean){
    await writeFile(configFile,`listen = "127.0.0.1:48184"\nbase_path = "/panel"\npublic_url = "${origin}"\ndata_dir = "${scratch}/data"\n[auth]\ndisabled = ${disabled}\nusername = "${ADMIN_USERNAME}"\npassword_hash = "${hash}"\n[telemt]\nurl = "${MOCK_URL}"\n[store]\ndriver = "memory"\n`);
    panelProcess=spawn(binary,["--config",configFile],{stdio:["ignore","ignore","pipe"]});
    panelProcess.stderr?.on("data",chunk=>{logs+=chunk.toString();});
    await expect.poll(async()=>{try{return(await fetch(base+"/api/health")).status;}catch{return 0;}}).toBe(200);
  }
  try{
    await start(true);
    const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
    for(const width of [390,1280]){
      await page.setViewportSize({width,height:900});await page.goto(base+"/login");
      await expect(page).toHaveURL(base+"/people");
      await expect(page.getByTestId("auth-disabled-notice")).toBeVisible();
      await expect(page.getByTestId("user-card-alice")).toBeVisible();
      await page.goto(base+"/server/settings");
      await expect(page.getByTestId("auth-disabled-details")).toBeVisible();
      await expect(page.getByTestId("settings-sessions")).toHaveCount(0);
      await expect(page.getByRole("button",{name:"Выйти",exact:true})).toHaveCount(0);
      await expect(page.getByRole("button",{name:"Добавить passkey",exact:true})).toHaveCount(0);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
      await page.screenshot({path:testInfo.outputPath(`anonymous-${width}.png`),fullPage:true});
    }
    expect((await page.context().cookies()).filter(c=>c.name.includes("session"))).toHaveLength(0);
    const me=await(await page.request.get(base+"/api/auth/me")).json();expect(me.auth_disabled).toBe(true);expect(me.username).toBe("anonymous");
    expect((await page.request.get(base+"/api/auth/sessions")).status()).toBe(403);
    expect((await page.request.post(base+"/api/auth/webauthn/register/begin",{headers:{"Sec-Fetch-Site":"same-origin"},data:{name:"blocked"}})).status()).toBe(403);
    expect((await page.request.put(base+"/api/settings/branding",{headers:{"Sec-Fetch-Site":"cross-site",Origin:"https://attacker.example"},data:{}})).status()).toBe(403);
    expect(logs).toContain("authentication is DISABLED");
    await stop();await start(false);await page.goto(base+"/people");
    await expect(page.getByLabel("Имя пользователя")).toBeVisible();
    await page.getByLabel("Имя пользователя").fill(ADMIN_USERNAME);await page.getByLabel("Пароль").fill(ADMIN_PASSWORD);
    await page.getByRole("button",{name:"Войти",exact:true}).click();await expect(page).toHaveURL(base+"/people");
    await expect(page.getByTestId("auth-disabled-notice")).toHaveCount(0);expect(errors).toEqual([]);
  }finally{await stop();await rm(scratch,{recursive:true,force:true});}
});
