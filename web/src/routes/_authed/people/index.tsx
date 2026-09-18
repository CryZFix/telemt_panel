import { createFileRoute } from "@tanstack/react-router";
import { PeopleList } from "../../../people/PeopleList";
import { NewUserPage } from "../../../people/NewUserPage";
import { QuotaSchedulePage } from "../../../people/QuotaSchedule";

export const Route = createFileRoute("/_authed/people/")({
  validateSearch:(search:Record<string,unknown>):{create?:boolean;schedule?:boolean}=>({create:search["create"]===true||search["create"]==="true"?true:undefined,schedule:search["schedule"]===true||search["schedule"]==="true"?true:undefined}),
  component: PeopleIndex,
});

function PeopleIndex(){const {create,schedule}=Route.useSearch();return create?<NewUserPage/>:schedule?<QuotaSchedulePage/>:<PeopleList/>;}
