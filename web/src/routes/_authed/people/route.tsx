import {createFileRoute, Outlet} from "@tanstack/react-router";
import {PeopleProvider} from "../../../people/PeopleContext";
import {BulkQuotaProvider} from "../../../people/BulkQuotaProvider";
import "../../../people/workspace.css";

export const Route = createFileRoute("/_authed/people")({
  component: ()=><PeopleProvider><BulkQuotaProvider><Outlet/></BulkQuotaProvider></PeopleProvider>,
});
