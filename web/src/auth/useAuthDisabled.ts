import {useQuery} from "@tanstack/react-query";
import {getMeOptions} from "../lib/api/generated/@tanstack/react-query.gen";

export function useAuthDisabled() {
  return useQuery(getMeOptions()).data?.auth_disabled === true;
}
