import { createFileRoute } from "@tanstack/react-router";
import { PersonDetail } from "../../../people/PersonDetail";

export const Route = createFileRoute("/_authed/people/$username")({
  validateSearch: (search:Record<string,unknown>):{tab?:"overview"|"access"|"ips"|"settings"}=>({tab:["overview","access","ips","settings"].includes(String(search["tab"]))?search["tab"] as "overview"|"access"|"ips"|"settings":undefined}),
  component: RouteComponent,
});

function RouteComponent() {
  const { username } = Route.useParams();
  const {tab}=Route.useSearch();
  return <PersonDetail key={username} username={username} tab={tab??"overview"} />;
}
