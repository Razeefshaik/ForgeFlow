import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

test("contribution execution evidence and PR approval gate", async ({page}) => {
  test.setTimeout(60_000);
  // Explicit UI fixtures; no external execution, fork, push or PR takes place.
  const c={id:"execution-ui",repository:"fixture/addition",title:"Correct addition",state:"PR_PREPARED",branch:"autopilot/issue-1",config_version:1,demo:false,workspace:"S:/StudyResource/TechBoooo/Backend/Contriss/execution-ui/repo",base_commit:"a".repeat(40)};
  const r={status:"PR_PREPARED",summary:"Corrected addition with regression coverage",fix_iterations:1,review_cycles:1,network:false,plan:{summary:"Fix addition",root_cause:"Incorrect subtraction",strategy:"Use addition and cover negative operands",files:["sum.go"],risks:[],unknowns:[]},review:{verdict:"APPROVE",summary:"Independent review approved",findings:[]},diff:"diff --git a/sum.go b/sum.go\n@@ -1 +1 @@\n-return a-b\n+return a+b\n",report:"Actual test evidence attached",pr_title:"Fix addition",pr_body:"Regression tests passed. Fixes #1",submission_token:"reviewed-token",pr_url:""};
  await page.route("**/api/contributions",route=>route.fulfill({json:[c]}));
  await page.route("**/api/contributions/execution-ui",route=>route.fulfill({json:c}));
  await page.route("**/api/contributions/execution-ui/execution",route=>route.fulfill({json:r}));
  await page.route("**/api/contributions/execution-ui/tests",route=>route.fulfill({json:[{id:"test",command:{program:"go",arguments:["test","./..."]},exit_code:0,output:"ok example.com/fixture",started_at:"2026-10-06T10:00:00Z",finished_at:"2026-10-06T10:00:02Z",truncated:false}]}));
  let submitted=0;
  await page.route("**/api/contributions/execution-ui/submit-pr",async route=>{expect(route.request().postDataJSON()).toMatchObject({approved:true,token:"reviewed-token"});submitted++;await route.fulfill({json:{...r,pr_url:"https://github.com/fixture/addition/pull/2"}})});
  await page.goto("/contributions");
  await page.getByRole("link",{name:/fixture\/addition/}).click();
  await page.getByRole("tab",{name:"Tests (1)"}).click();
  await expect(page.getByText(/go test \.\/.*exit 0/)).toBeVisible();
  await page.getByRole("tab",{name:"Review",exact:true}).click();
  await expect(page.getByText("Independent review approved",{exact:true})).toBeVisible();
  await page.getByRole("tab",{name:"Diff",exact:true}).click();
  await expect(page.locator(".diff-add")).toContainText("return a+b");
  const artifacts=fileURLToPath(new URL("../../../artifacts/",import.meta.url));
  await page.screenshot({path:path.join(artifacts,"execution-desktop.png"),fullPage:true,animations:"disabled",caret:"hide"});
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
  await page.screenshot({path:path.join(artifacts,"execution-mobile.png"),fullPage:true,animations:"disabled",caret:"hide"});
  await page.getByRole("tab",{name:"PR",exact:true}).click();
  const submit=page.getByRole("button",{name:"Submit PR to GitHub"});await expect(submit).toBeDisabled();expect(submitted).toBe(0);
  await page.getByRole("checkbox",{name:/I approve creating or using my fork/}).check();
  await expect(submit).toBeEnabled();await submit.click();expect(submitted).toBe(1);
});
