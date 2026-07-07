import { axiosClient } from "@/shared/api/axiosClient";

// LinkedIdentity is the safe per-provider projection returned by the profile
// identity endpoints — never the internal row id or provider subject.
export interface LinkedIdentity {
  provider: string;
  email: string;
  createdAt: string;
}

// LinkedAccountsResponse is the GET /users/:id/identities payload. hasPassword
// tells the client whether the account can still be signed into without any
// linked identity — it drives the "cannot unlink your last method" UX.
export interface LinkedAccountsResponse {
  hasPassword: boolean;
  identities: LinkedIdentity[];
}

export interface LinkIdentityRequest {
  credential: string;
}

// axiosClient (authed) unwraps the { data } envelope and throws FetchError on
// error, so these clients stay thin — no manual mapping (unlike auth.ts, which
// uses axiosPublic).
export function getIdentities(userId: number): Promise<LinkedAccountsResponse> {
  return axiosClient.get(`/users/${userId}/identities`);
}

export function linkIdentity(
  userId: number,
  provider: string,
  data: LinkIdentityRequest,
): Promise<LinkedIdentity> {
  return axiosClient.post(`/users/${userId}/identities/${provider}`, data);
}

export function unlinkIdentity(userId: number, provider: string): Promise<void> {
  return axiosClient.delete(`/users/${userId}/identities/${provider}`);
}
