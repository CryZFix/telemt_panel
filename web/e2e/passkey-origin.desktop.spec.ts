import {test,expect} from "./fixtures";

test.use({serviceWorkers:"block"});

test("proxy origin mismatch is explained before the authenticator creates a key",async({page,login})=>{
  await login();
  const cdp=await page.context().newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  const {authenticatorId}=await cdp.send("WebAuthn.addVirtualAuthenticator",{options:{protocol:"ctap2",transport:"internal",hasResidentKey:true,hasUserVerification:true,isUserVerified:true,automaticPresenceSimulation:true}});
  // The request still reaches the real handler: simulate the HTTPS origin a
  // TLS proxy would forward to an untrusted HTTP backend, not an API error stub.
  await page.route("**/api/auth/webauthn/register/begin",async route=>{
    const response=await route.fetch({headers:{...await route.request().allHeaders(),origin:"https://localhost:48180","sec-fetch-site":"same-origin"}});
    await route.fulfill({response});
  });
  await page.goto("/server/settings");
  await page.getByRole("button",{name:"Добавить passkey",exact:true}).click();
  const dialog=page.getByRole("dialog",{name:"Добавить passkey",exact:true});
  await dialog.getByLabel("Название passkey").fill("Must not be created");
  await dialog.getByRole("button",{name:"Продолжить",exact:true}).click();
  await expect(dialog.getByRole("alert")).toContainText("trusted_proxies");
  const {credentials}=await cdp.send("WebAuthn.getCredentials",{authenticatorId});
  expect(credentials).toHaveLength(0);
  await cdp.detach();
});
