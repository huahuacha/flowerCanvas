import { useQuery } from "@tanstack/react-query";
import { Alert, App, Button, Form, Input, Table, Tag } from "antd";
import { UserPlus } from "lucide-react";
import { nanoid } from "nanoid";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { AppModal } from "@/components/ui/product/app-modal";
import { formatCredits } from "@/constant/credits";
import { allocateCompanyCredits, createCompanyMember, disableCompanyMember, getCompanyMembers, type CompanyMember, type CreateCompanyMemberInput } from "@/services/api/company";
import { ApiError } from "@/services/api/request";
import { useUserStore } from "@/stores/use-user-store";

type PendingAllocation = { requestId: string; amountMicrocredits: number; userId: string };

function parseMicrocredits(amount: string) {
    const value = amount.trim();
    if (!/^\d+(?:\.\d{1,6})?$/.test(value)) return NaN;
    const [whole, fraction = ""] = value.split(".");
    return Number(whole + fraction.padEnd(6, "0"));
}

export function CompanyMembersPane() {
    const { t } = useTranslation();
    const { message, modal } = App.useApp();
    const userId = useUserStore((state) => state.user?.id);
    const members = useQuery({ queryKey: ["company-members", userId], queryFn: ({ signal }) => getCompanyMembers(signal), enabled: Boolean(userId) });
    const [createOpen, setCreateOpen] = useState(false);
    const [allocating, setAllocating] = useState<CompanyMember | null>(null);
    const [retryAllocation, setRetryAllocation] = useState<PendingAllocation | null>(null);
    const [working, setWorking] = useState(false);
    const submitting = useRef(false);

    const create = async (fields: CreateCompanyMemberInput) => {
        if (submitting.current) return;
        submitting.current = true;
        setWorking(true);
        try {
            await createCompanyMember({ ...fields, username: fields.username.trim(), displayName: fields.displayName?.trim(), email: fields.email?.trim() });
            setCreateOpen(false);
            message.success(t("company.members.created"));
            await members.refetch();
        } catch (error) {
            message.error(error instanceof Error ? error.message : t("company.members.createFailed"));
        } finally {
            submitting.current = false;
            setWorking(false);
        }
    };

    const allocate = async ({ amount }: { amount: string }) => {
        if (!allocating || submitting.current) return;
        const request = retryAllocation ?? { userId: allocating.userId, amountMicrocredits: parseMicrocredits(amount), requestId: nanoid() };
        submitting.current = true;
        setWorking(true);
        try {
            await allocateCompanyCredits(request.userId, { amountMicrocredits: request.amountMicrocredits, requestId: request.requestId });
            setRetryAllocation(null);
            setAllocating(null);
            message.success(t("company.members.allocated", { amount: formatCredits(request.amountMicrocredits, 6) }));
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            await members.refetch();
        } catch (error) {
            setRetryAllocation(!(error instanceof ApiError) || error.retryable ? request : null);
            message.error(error instanceof Error ? error.message : t("company.members.allocateFailed"));
        } finally {
            submitting.current = false;
            setWorking(false);
        }
    };

    const disable = (member: CompanyMember) => {
        modal.confirm({
            title: t("company.members.disableTitle", { name: member.displayName || member.username }),
            content: t("company.members.disableDescription"),
            okText: t("company.members.disable"),
            cancelText: t("common.cancel"),
            okButtonProps: { danger: true },
            onOk: async () => {
                try {
                    await disableCompanyMember(member.userId);
                    message.success(t("company.members.disabled"));
                    await members.refetch();
                } catch (error) {
                    message.error(error instanceof Error ? error.message : t("company.members.disableFailed"));
                    throw error;
                }
            },
        });
    };

    return (
        <>
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>{t("company.company-members.label")}</h2>
                    <p>{members.data ? t("company.members.balance", { company: members.data.company.companyName, amount: formatCredits(members.data.myAvailableMicrocredits, 6) }) : t("company.company-members.description")}</p>
                </div>
                <Button type="primary" disabled={!members.data || working} icon={<UserPlus className="size-4" />} onClick={() => setCreateOpen(true)}>
                    {t("company.members.add")}
                </Button>
            </div>
            <div className="settings-section">
                {members.isError && <Alert type="error" showIcon className="mb-4" title={members.error.message} action={<Button onClick={() => void members.refetch()}>{t("common.retry")}</Button>} />}
                <Table<CompanyMember>
                    rowKey="userId"
                    size="small"
                    className="app-data-table"
                    dataSource={members.data?.members ?? []}
                    loading={members.isFetching}
                    locale={{ emptyText: members.isError ? "—" : t("company.members.empty") }}
                    scroll={{ x: 620 }}
                    columns={[
                        {
                            title: t("company.members.username"),
                            key: "username",
                            render: (_, member) => (
                                <>
                                    {member.displayName || member.username}
                                    <div className="text-xs text-muted-foreground">@{member.username}</div>
                                </>
                            ),
                        },
                        { title: t("company.members.role"), key: "role", render: (_, member) => t(`company.members.roles.${member.role}`) },
                        { title: t("company.members.status"), key: "status", render: (_, member) => <Tag color={member.status === "active" ? "success" : "default"}>{t(`company.members.statuses.${member.status}`)}</Tag> },
                        { title: t("company.members.available"), dataIndex: "availableMicrocredits", align: "right", render: (value: number) => formatCredits(value, 6) },
                        { title: t("company.members.reserved"), dataIndex: "reservedMicrocredits", align: "right", render: (value: number) => formatCredits(value, 6) },
                        {
                            title: t("company.members.actions"),
                            key: "actions",
                            render: (_, member) =>
                                member.role === "employee" && (member.status === "active" || retryAllocation?.userId === member.userId) ? (
                                    <div className="flex flex-wrap gap-2">
                                        <Button size="small" disabled={Boolean(retryAllocation && retryAllocation.userId !== member.userId)} onClick={() => setAllocating(member)}>
                                            {retryAllocation?.userId === member.userId ? t("company.members.retryButton") : t("company.members.allocate")}
                                        </Button>
                                        <Button size="small" danger disabled={member.status !== "active" || retryAllocation?.userId === member.userId} onClick={() => disable(member)}>
                                            {t("company.members.disable")}
                                        </Button>
                                    </div>
                                ) : null,
                        },
                    ]}
                    pagination={{ pageSize: 20, hideOnSinglePage: true }}
                />
            </div>
            <AppModal
                open={createOpen}
                title={t("company.members.add")}
                okText={t("company.members.create")}
                cancelText={t("common.cancel")}
                confirmLoading={working}
                okButtonProps={{ htmlType: "submit", form: "company-create-member" }}
                onCancel={() => !working && setCreateOpen(false)}
            >
                <p className="mb-4 text-sm text-muted-foreground">{t("company.members.startingBalance")}</p>
                <Form<CreateCompanyMemberInput> id="company-create-member" layout="vertical" onFinish={create} disabled={working}>
                    <Form.Item name="username" label={t("company.members.username")} rules={[{ required: true, whitespace: true }]}>
                        <Input autoComplete="off" maxLength={32} />
                    </Form.Item>
                    <Form.Item name="displayName" label={t("company.members.displayName")}>
                        <Input autoComplete="off" maxLength={80} />
                    </Form.Item>
                    <Form.Item name="email" label={t("company.members.email")} rules={[{ type: "email" }]}>
                        <Input type="email" autoComplete="off" maxLength={160} />
                    </Form.Item>
                    <Form.Item name="password" label={t("company.members.password")} rules={[{ required: true }]}>
                        <Input.Password autoComplete="new-password" maxLength={72} />
                    </Form.Item>
                </Form>
            </AppModal>
            <AppModal
                open={Boolean(allocating)}
                title={t("company.members.allocate")}
                okText={retryAllocation ? t("common.retry") : t("company.members.allocate")}
                cancelText={t("common.cancel")}
                confirmLoading={working}
                okButtonProps={{ htmlType: "submit", form: "company-allocate-credits" }}
                onCancel={() => {
                    if (working) return;
                    setAllocating(null);
                }}
            >
                {allocating && <p className="mb-4 text-sm">{t("company.members.recipient", { name: allocating.displayName || allocating.username })}</p>}
                {retryAllocation && <Alert className="mb-4" type="warning" title={t("company.members.retryAllocation")} />}
                <Form<{ amount: string }>
                    id="company-allocate-credits"
                    layout="vertical"
                    initialValues={
                        retryAllocation
                            ? {
                                  amount: String(retryAllocation.amountMicrocredits)
                                      .padStart(7, "0")
                                      .replace(/(\d{6})$/, ".$1"),
                              }
                            : undefined
                    }
                    onFinish={allocate}
                >
                    <Form.Item
                        name="amount"
                        label={t("company.members.amount")}
                        rules={[
                            {
                                validator: async (_, amount: string | undefined) => {
                                    if (retryAllocation) return;
                                    const microcredits = parseMicrocredits(amount ?? "");
                                    if (!Number.isSafeInteger(microcredits) || microcredits <= 0) throw new Error(t("company.members.invalidAmount"));
                                    if (microcredits > (members.data?.myAvailableMicrocredits ?? 0)) throw new Error(t("company.members.insufficientBalance"));
                                },
                            },
                        ]}
                    >
                        <Input inputMode="decimal" disabled={working || Boolean(retryAllocation)} />
                    </Form.Item>
                </Form>
            </AppModal>
        </>
    );
}
