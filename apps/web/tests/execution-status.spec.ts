import {test,expect} from "@playwright/test";
test("running contribution displays current phase instead of previous failure",async({page})=>{
 const c={id:"status-ui",repository:"fixture/status",title:"Fix status",state:"FIXING",branch:"autopilot/issue-1",config_version:1,demo:false,message:"Execution running: fixing"};
 await page.route("**/api/contributions",r=>r.fulfill({json:[c]}));
 await page.route("**/api/contributions/status-ui",r=>r.fulfill({json:c}));
 await page.route("**/api/contributions/status-ui/execution",r=>r.fulfill({json:{status:"RUNNING",phase:"FIXING",message:"Execution running: fixing",summary:"contributor changed Git history; inspect workspace before continuing",fix_iterations:1,review_cycles:0,network:false}}));
 await page.route("**/api/contributions/status-ui/tests",r=>r.fulfill({json:[]}));
 await page.goto("/contributions");await page.getByRole("link",{name:/fixture\/status/}).click();
 await expect(page.locator('.execution-panel').getByText("Execution running: fixing",{exact:true})).toBeVisible();
 await expect(page.getByText("contributor changed Git history; inspect workspace before continuing",{exact:true})).toHaveCount(0);
 await expect(page.getByRole("button",{name:"Pause contribution"})).toBeVisible();
});
