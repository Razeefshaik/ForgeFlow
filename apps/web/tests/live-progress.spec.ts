import{test,expect}from"@playwright/test";
test("live activity advances beyond an old entity page and polls without SSE",async({page})=>{
 const c={id:"live-ui",repository:"fixture/live",title:"Show fresh progress",state:"FIXING",branch:"autopilot/issue-1",config_version:1,demo:false,message:"Execution running: fixing"};
 const event=(id:number,message:string)=>({id,type:"AgentActivity",entity_id:c.id,actor:"contributor",message,data:{},created_at:new Date().toISOString(),demo:false});
 let recent=[event(200,"Newest activity past first 100 events")];
 await page.route("**/api/events/stream",r=>r.abort());
 await page.route("**/api/events",r=>r.fulfill({json:recent}));
 await page.route("**/api/events?entity=*",r=>r.fulfill({json:[event(1,"Old first-page event")]}));
 await page.route("**/api/contributions",r=>r.fulfill({json:[c]}));
 await page.route("**/api/contributions/live-ui",r=>r.fulfill({json:c}));
 await page.route("**/api/contributions/live-ui/execution",r=>r.fulfill({json:{status:"RUNNING",phase:"FIXING",message:c.message,summary:"",fix_iterations:1,review_cycles:0,network:false}}));
 await page.route("**/api/contributions/live-ui/tests",r=>r.fulfill({json:[]}));
 await page.route("**/api/agents",r=>r.fulfill({json:[{id:"active-run",contribution_id:c.id,role:"contributor",status:"RUNNING",started_at:new Date(Date.now()-120000).toISOString()}]}));
 await page.goto("/contributions");await page.getByRole("link",{name:/fixture\/live/}).click();
 await expect(page.getByLabel("Live execution progress")).toContainText("Codex contributor process");
 await expect(page.locator(".event-list").getByText("Newest activity past first 100 events",{exact:true})).toBeVisible();
 recent=[event(201,"Updated through polling with SSE disconnected")];
 await expect(page.locator(".event-list").getByText("Updated through polling with SSE disconnected",{exact:true})).toBeVisible({timeout:10000});
});
