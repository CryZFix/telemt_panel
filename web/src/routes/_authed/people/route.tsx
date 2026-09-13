import {createFileRoute, Outlet} from "@tanstack/react-router";
import {PeopleProvider} from "../../../people/PeopleContext";
import "../../../people/workspace.css";

export const Route = createFileRoute("/_authed/people")({
  component: ()=><PeopleProvider><Outlet/></PeopleProvider>,
});
