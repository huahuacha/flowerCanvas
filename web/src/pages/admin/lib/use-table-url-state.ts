import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";

export type TableUrlState = {
    filter: string;
    role: string;
    status: string;
    /** 用户分组编码；all 表示不过滤。 */
    group: string;
    companyId: string;
    companyRole: "all" | "admin" | "employee" | "none";
    page: number;
    pageSize: number;
};

function positiveInteger(value: string | null, fallback: number) {
    const parsed = Number(value);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function allowedValue<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
    return allowed.find((option) => option === value) ?? fallback;
}

export function useTableUrlState(defaultPageSize = 20) {
    const [searchParams, setSearchParams] = useSearchParams();
    const state = useMemo<TableUrlState>(
        () => ({
            filter: searchParams.get("filter") || "",
            role: allowedValue(searchParams.get("role"), ["all", "admin", "user"], "all"),
            status: allowedValue(searchParams.get("status"), ["all", "active", "disabled"], "all"),
            group: /^[a-z0-9][a-z0-9_-]{0,39}$/.test(searchParams.get("group") || "") ? searchParams.get("group")! : "all",
            companyId: searchParams.get("companyId") || "",
            companyRole: allowedValue(searchParams.get("companyRole"), ["all", "admin", "employee", "none"], "all"),
            page: positiveInteger(searchParams.get("page"), 1),
            pageSize: [20, 50, 100].includes(positiveInteger(searchParams.get("pageSize"), defaultPageSize)) ? positiveInteger(searchParams.get("pageSize"), defaultPageSize) : defaultPageSize,
        }),
        [defaultPageSize, searchParams],
    );

    const update = useCallback(
        (patch: Partial<TableUrlState>, replace = false) => {
            const next = new URLSearchParams(searchParams);
            const merged = { ...state, ...patch };
            const defaults: TableUrlState = { filter: "", role: "all", status: "all", group: "all", companyId: "", companyRole: "all", page: 1, pageSize: defaultPageSize };
            (Object.keys(defaults) as Array<keyof TableUrlState>).forEach((key) => {
                const value = merged[key];
                if (value === defaults[key]) next.delete(key);
                else next.set(key, String(value));
            });
            setSearchParams(next, { replace });
        },
        [defaultPageSize, searchParams, setSearchParams, state],
    );

    return { state, update };
}
