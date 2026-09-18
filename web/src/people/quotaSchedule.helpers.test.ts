import {describe,it,expect} from "vitest";
import {formatScheduleRun} from "./quotaSchedule.helpers";
describe("quota schedule dates",()=>{
 it("shows the server wall date, not the browser date",()=>{
   expect(formatScheduleRun("2026-09-13T23:30:00-07:00","America/Los_Angeles","en")).toContain("Sep 13");
   expect(formatScheduleRun("2026-09-13T23:30:00-07:00","UTC","en")).toContain("Sep 14");
 });
 it("retains the offset for a server without a named IANA zone",()=>{
   const value=formatScheduleRun("2026-09-13T23:30:00-07:00","Local","en");expect(value).toContain("Sep 13");expect(value).toContain("23:30");expect(value).toContain("-07:00");
 });
});
