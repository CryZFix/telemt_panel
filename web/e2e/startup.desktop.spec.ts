import {test,expect,LOCALE_STORAGE_KEY} from "./fixtures";

test.use({serviceWorkers:"block"});

for(const locale of ["ru","en"] as const){
  for(const width of [320,1280]){
    test(`module failure has a standalone recovery screen (${locale}, ${width}px)`,async({page},testInfo)=>{
      await page.addInitScript(({key,locale})=>localStorage.setItem(key,locale),{key:LOCALE_STORAGE_KEY,locale});
      await page.setViewportSize({width,height:900});
      let requests=0;
      await page.route("**/assets/**",route=>{requests++;return route.abort();});
      await page.goto("/login");
      const error=page.locator("#panel-bootstrap-error");
      await expect(error).toBeVisible();
      await expect(error.getByText(locale==="ru"?"Не удалось загрузить интерфейс":"The interface could not be loaded",{exact:true})).toBeVisible();
      await expect(error).toContainText("base_path");
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
      await page.screenshot({path:testInfo.outputPath(`startup-error-${locale}-${width}.png`)});
      const settled=requests;await page.waitForTimeout(700);expect(requests).toBe(settled);
      await page.unroute("**/assets/**");
      await error.getByRole("button",{name:locale==="ru"?"Повторить загрузку":"Reload",exact:true}).click();
      await expect(page.getByLabel(locale==="ru"?"Имя пользователя":"Username",{exact:true})).toBeVisible();
      await expect(page.locator("#panel-bootstrap")).toHaveCount(0);
    });
  }
}

test("a delayed module remains loading instead of reporting failure",async({page})=>{
  let release!:()=>void,held=false;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  await page.route("**/assets/*.js",async route=>{if(!held){held=true;await gate;}await route.continue();});
  try{
    await page.goto("/login",{waitUntil:"commit"});
    await expect.poll(()=>held).toBe(true);
    await expect(page.locator("#panel-bootstrap-loading")).toBeVisible();
    await page.waitForTimeout(1200);
    await expect(page.locator("#panel-bootstrap-error")).toBeHidden();
  }finally{release();}
  await expect(page.getByLabel("Имя пользователя",{exact:true})).toBeVisible();
  await expect(page.locator("#panel-bootstrap")).toHaveCount(0);
});

test("JavaScript disabled explains the requirement",async({browser},testInfo)=>{
  const context=await browser.newContext({javaScriptEnabled:false});
  try{const page=await context.newPage();await page.goto("http://localhost:48180/login");await page.screenshot({path:testInfo.outputPath("no-javascript.png")});const note=page.locator("noscript p");await expect(note).toBeVisible();expect(await note.textContent()).toContain("Enable JavaScript");}finally{await context.close();}
});
