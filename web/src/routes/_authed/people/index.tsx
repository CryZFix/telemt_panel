import { createFileRoute } from "@tanstack/react-router";
import { PeopleList } from "../../../people/PeopleList";
import { NewUserPage } from "../../../people/NewUserPage";

export const Route = createFileRoute("/_authed/people/")({
  validateSearch:(search:Record<string,unknown>):{create?:boolean}=>({create:search["create"]===true||search["create"]==="true"?true:undefined}),
  component: PeopleIndex,
});

function PeopleIndex(){const {create}=Route.useSearch();return create?<NewUserPage/>:<PeopleList/>;}
