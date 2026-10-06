import { test, expect } from "@playwright/test";

test("GitHub account setup, browser challenge, saved account and logout", async ({page})=>{
 let state:any={configured:false,client_id:"",connected:false,source:"public unauthenticated",login:"",pending:false,message:""};
 await page.route("**/api/auth/github",r=>r.fulfill({json:state}));
 await page.route("**/api/auth/github/configure",async r=>{expect(r.request().postDataJSON()).toEqual({client_id:"Ov23liForgeFlowTest"});state={...state,configured:true,client_id:"Ov23liForgeFlowTest"};await r.fulfill({json:state});});
 await page.route("**/api/auth/github/login",r=>{state={...state,pending:true,user_code:"TEST-CODE",verification_uri:"https://github.com/login/device",expires_at:new Date(Date.now()+900000).toISOString()};return r.fulfill({json:state});});
 await page.route("**/api/auth/github/logout",r=>{state={...state,connected:false,pending:false,login:"",message:"Signed out"};return r.fulfill({json:state});});
 await page.goto("/login");await expect(page.getByRole("heading",{name:"One-time OAuth app setup"})).toBeVisible();
 await page.getByLabel("OAuth Client ID").fill("Ov23liForgeFlowTest");await page.getByRole("button",{name:"Save Client ID"}).click();
 await page.getByRole("button",{name:"Sign in with GitHub",exact:true}).click();
 await expect(page.getByLabel("GitHub verification code")).toHaveText("TEST-CODE");await expect(page.getByRole("link",{name:"Continue on GitHub"})).toHaveAttribute("href","https://github.com/login/device");
 state={...state,pending:false,connected:true,login:"test-user",source:"GitHub sign-in",user_code:undefined};
 await expect(page.getByRole("heading",{name:"Connected as test-user"})).toBeVisible();
 await page.reload();await expect(page.getByRole("heading",{name:"Connected as test-user"})).toBeVisible();
 await page.getByRole("button",{name:"Sign out of ForgeFlow"}).click();await expect(page.getByRole("button",{name:"Sign in with GitHub",exact:true})).toBeVisible();
});
