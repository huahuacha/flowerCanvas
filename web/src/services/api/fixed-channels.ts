import { http } from "@/services/api/request";

export type FixedChannel = {
    id: string;
    name: string;
    enabled: boolean;
    hasCredential: boolean;
    credentialUpdatedAt?: string;
};

export const listFixedChannels = (signal?: AbortSignal) => http.get<{ channels: FixedChannel[] }>("/fixed-channels", { signal });
export const saveFixedChannelCredential = (channelId: string, apiKey: string) => http.put<FixedChannel>(`/fixed-channels/${encodeURIComponent(channelId)}/credential`, { apiKey });
export const deleteFixedChannelCredential = (channelId: string) => http.delete<{ ok: boolean }>(`/fixed-channels/${encodeURIComponent(channelId)}/credential`);
