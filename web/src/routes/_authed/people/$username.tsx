import { createFileRoute } from "@tanstack/react-router";
import { PersonDetail, type PersonTab } from "../../../people/PersonDetail";

export const Route = createFileRoute("/_authed/people/$username")({
  validateSearch: (search:Record<string,unknown>):{tab?:PersonTab}=>({tab:["overview","access","ips","schedule","settings"].includes(String(search["tab"]))?search["tab"] as PersonTab:undefined}),
  component: RouteComponent,
});

function RouteComponent() {
  const { username } = Route.useParams();
  const {tab}=Route.useSearch();
  return <PersonDetail key={username} username={username} tab={tab??"overview"} />;
}
