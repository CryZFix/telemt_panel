// Keep the selected schedule's wall time even if the browser is elsewhere.
export function formatScheduleRun(value:string,zone:string,locale:string):string {
  if(zone!=="Local")return new Intl.DateTimeFormat(locale,{timeZone:zone,day:"numeric",month:"short",year:"numeric",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).format(new Date(value));
  const wall=value.slice(0,19)+"Z",offset=value.slice(19);
  return `${new Intl.DateTimeFormat(locale,{timeZone:"UTC",day:"numeric",month:"short",year:"numeric",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).format(new Date(wall))} ${offset}`;
}
