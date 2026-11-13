import { http } from "@/services/api/request";

export type CompanyIdentity = {
    companyId: string;
    companyName: string;
    role: "admin" | "employee";
    isAdmin: boolean;
};

export type CompanyMember = {
    userId: string;
    username: string;
    displayName: string;
    email?: string;
    role: CompanyIdentity["role"];
    status: "active" | "disabled";
    availableMicrocredits: number;
    reservedMicrocredits: number;
    joinedAt: string;
};

export type CompanyMembersView = {
    company: CompanyIdentity;
    members: CompanyMember[];
    myAvailableMicrocredits: number;
    myReservedMicrocredits: number;
};

export type CreateCompanyMemberInput = {
    username: string;
    displayName?: string;
    email?: string;
    password: string;
};

type CompanyCreditAccount = Pick<CompanyMember, "userId" | "availableMicrocredits" | "reservedMicrocredits">;

export type AdminCompany = {
    id: string;
    name: string;
    memberCount: number;
    adminCount: number;
    admins: Pick<CompanyMember, "userId" | "username" | "displayName" | "role" | "status">[];
    createdAt: string;
};

export const getCompanyMembership = (signal?: AbortSignal) => http.get<{ company: CompanyIdentity | null }>("/company/membership", { signal });
export const getCompanyMembers = (signal?: AbortSignal) => http.get<CompanyMembersView>("/company/members", { signal });
export const createCompanyMember = (input: CreateCompanyMemberInput) => http.post<{ member: CompanyMember }>("/company/members", input);
export const disableCompanyMember = (userId: string) => http.post<{ member: CompanyMember }>(`/company/members/${encodeURIComponent(userId)}/disable`);
export const allocateCompanyCredits = (userId: string, input: { amountMicrocredits: number; requestId: string }) =>
    http.post<{ actorAccount: CompanyCreditAccount; targetAccount: CompanyCreditAccount; replayed: boolean }>(`/company/members/${encodeURIComponent(userId)}/credits`, input);
export const listAdminCompanies = (signal?: AbortSignal) => http.get<{ companies: AdminCompany[] }>("/admin/companies", { signal });
export const createAdminCompany = (input: { name: string; adminUsername: string }) => http.post<{ company: AdminCompany }>("/admin/companies", input);
