import { useQuery } from "@tanstack/react-query";
import { Alert, App, Button, Form, Input, Table } from "antd";
import { Plus } from "lucide-react";
import { useRef, useState } from "react";

import { Select } from "@/components/ui/base/select";
import { deleteAdminUser, updateAdminUser } from "@/services/api/auth";
import { addAdminCompanyAdmin, createAdminCompany, demoteAdminCompanyAdmin, listAdminCompanies, updateAdminCompanyStatus, type AdminCompany, type CompanyMemberBrief } from "@/services/api/company";
import { DEFAULT_USER_GROUP_CODE, listAdminUserGroups } from "@/services/api/user-groups";
import { useUserStore } from "@/stores/use-user-store";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminDataTable, AdminStatusBadge, AdminTableEmpty } from "../components/admin-ui";
import { AdminUserDetailDrawer } from "../components/admin-user-detail-drawer";
import { AdminModal } from "../ui/overlays";

type CompanyFields = Parameters<typeof createAdminCompany>[0];

export default function CompaniesPage() {
    const { message, modal } = App.useApp();
    const userId = useUserStore((state) => state.user?.id);
    const companies = useQuery({ queryKey: ["admin-companies", userId], queryFn: ({ signal }) => listAdminCompanies(signal), enabled: Boolean(userId) });
    const [createOpen, setCreateOpen] = useState(false);
    const groups = useQuery({ queryKey: ["company-admin-user-groups", userId], queryFn: () => listAdminUserGroups(), enabled: createOpen && Boolean(userId) });
    const [detailUserId, setDetailUserId] = useState<string | null>(null);
    const [addingTo, setAddingTo] = useState<AdminCompany | null>(null);
    const [saving, setSaving] = useState(false);
    const submitting = useRef(false);

    const create = async ({ name, admin }: CompanyFields) => {
        if (submitting.current) return;
        submitting.current = true;
        setSaving(true);
        try {
            await createAdminCompany({ name: name.trim(), admin: { ...admin, username: admin.username.trim(), email: admin.email?.trim() || undefined } });
            setCreateOpen(false);
            message.success("公司已开通，可在用户管理中为公司管理员发放积分");
            await companies.refetch();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "开通公司失败");
        } finally {
            submitting.current = false;
            setSaving(false);
        }
    };

    const confirmChange = (title: string, content: string, change: () => Promise<unknown>, danger = false) => {
        modal.confirm({
            title,
            content,
            okText: "确认",
            cancelText: "取消",
            okButtonProps: { danger },
            onOk: async () => {
                try {
                    await change();
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "操作失败");
                    throw error;
                }
                message.success("已更新");
                await companies.refetch();
            },
        });
    };

    const addAdmin = async ({ usernameOrEmail }: { usernameOrEmail: string }) => {
        if (!addingTo || submitting.current) return;
        submitting.current = true;
        setSaving(true);
        try {
            await addAdminCompanyAdmin(addingTo.id, usernameOrEmail.trim());
            setAddingTo(null);
            message.success("已添加公司管理员");
            await companies.refetch();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "添加公司管理员失败");
        } finally {
            submitting.current = false;
            setSaving(false);
        }
    };

    return (
        <AdminPageFrame
            title="公司管理"
            description="开通公司、管理公司管理员和查看成员。"
            actions={
                <Button type="primary" icon={<Plus className="size-4" />} onClick={() => setCreateOpen(true)}>
                    开通公司
                </Button>
            }
        >
            <div className="admin-panel">
                {companies.isError && <Alert type="error" showIcon title={companies.error.message} action={<Button onClick={() => void companies.refetch()}>重试</Button>} />}
                <AdminDataTable<AdminCompany>
                    skeletonColumns={4}
                    empty={<AdminTableEmpty title={companies.isError ? "公司列表加载失败" : "暂无公司"} />}
                    table={{
                        className: "app-data-table",
                        size: "small",
                        rowKey: "id",
                        loading: companies.isFetching,
                        dataSource: companies.data?.companies ?? [],
                        scroll: { x: 900 },
                        pagination: { pageSize: 20, hideOnSinglePage: true },
                        expandable: {
                            columnTitle: "成员名单",
                            rowExpandable: (company) => company.memberCount > 0,
                            expandIcon: ({ expanded, onExpand, record, expandable }) =>
                                expandable ? (
                                    <Button type="link" size="small" aria-expanded={expanded} aria-label={`${expanded ? "收起" : "展开"}${record.name}的成员`} onClick={(event) => onExpand(record, event)}>
                                        {expanded ? "收起成员" : "展开成员"}
                                    </Button>
                                ) : null,
                            expandedRowRender: (company) => (
                                <Table<CompanyMemberBrief>
                                    rowKey="userId"
                                    size="small"
                                    dataSource={company.members}
                                    pagination={false}
                                    columns={[
                                        { title: "姓名", key: "name", render: (_, member) => member.displayName || member.username },
                                        { title: "用户名", dataIndex: "username" },
                                        { title: "公司身份", key: "role", render: (_, member) => <AdminStatusBadge label={member.role === "admin" ? "公司管理员" : "员工"} tone={member.role === "admin" ? "info" : "neutral"} /> },
                                        { title: "账号状态", key: "status", render: (_, member) => (member.status === "disabled" ? "已停用" : "已启用") },
                                        {
                                            title: "操作",
                                            key: "actions",
                                            render: (_, member) => {
                                                const active = member.status === "active";
                                                const name = member.displayName || member.username;
                                                return (
                                                    <div className="flex flex-wrap gap-1">
                                                        <Button type="link" size="small" onClick={() => setDetailUserId(member.userId)}>
                                                            查看详情
                                                        </Button>
                                                        {member.userId !== userId && (
                                                            <Button
                                                                type="link"
                                                                size="small"
                                                                danger={active}
                                                                onClick={() =>
                                                                    confirmChange(
                                                                        `${active ? "停用" : "启用"} ${name} 的账号？`,
                                                                        active ? "该成员的登录会话将失效。" : "账号将恢复启用，访问权限仍受公司状态限制。",
                                                                        () => (active ? deleteAdminUser(member.userId) : updateAdminUser(member.userId, { status: "active" })),
                                                                        active,
                                                                    )
                                                                }
                                                            >
                                                                {active ? "停用账号" : "启用账号"}
                                                            </Button>
                                                        )}
                                                        {member.role === "admin" && (
                                                            <Button type="link" size="small" onClick={() => confirmChange(`将 ${name} 改为公司员工？`, "该成员将失去公司管理权限。", () => demoteAdminCompanyAdmin(company.id, member.userId))}>
                                                                改为员工
                                                            </Button>
                                                        )}
                                                    </div>
                                                );
                                            },
                                        },
                                    ]}
                                />
                            ),
                        },
                        columns: [
                            { title: "公司", dataIndex: "name" },
                            {
                                title: "公司状态",
                                key: "status",
                                render: (_, company) => <AdminStatusBadge label={company.status === "active" ? "已启用" : company.status === "disabled" ? "已停用" : "状态未提供"} tone={company.status === "active" ? "success" : "neutral"} />,
                            },
                            {
                                title: "公司管理员",
                                key: "admins",
                                render: (_, company) =>
                                    company.admins.map((admin) => (
                                        <div key={admin.userId}>
                                            {admin.displayName || admin.username}{" "}
                                            <span className="text-muted-foreground">
                                                @{admin.username}
                                                {admin.status === "disabled" && " · 已停用"}
                                            </span>
                                        </div>
                                    )),
                            },
                            { title: "成员构成", key: "memberCount", render: (_, company) => `${company.adminCount} 位管理员 · ${company.memberCount - company.adminCount} 位员工` },
                            { title: "开通时间", dataIndex: "createdAt", render: (date: string) => new Date(date).toLocaleString() },
                            {
                                title: "操作",
                                key: "actions",
                                render: (_, company) => {
                                    const disabled = company.status === "disabled";
                                    return (
                                        <div className="flex flex-wrap gap-1">
                                            <Button type="link" size="small" onClick={() => setAddingTo(company)}>
                                                添加管理员
                                            </Button>
                                            <Button
                                                type="link"
                                                size="small"
                                                danger={!disabled}
                                                disabled={!company.status}
                                                onClick={() =>
                                                    confirmChange(
                                                        `${disabled ? "恢复" : "停用"} ${company.name}？`,
                                                        disabled ? "已启用的成员可以重新登录，单独停用的账号继续停用。" : "所有管理员和员工将无法登录，已有登录会话将失效。",
                                                        () => updateAdminCompanyStatus(company.id, disabled ? "active" : "disabled"),
                                                        !disabled,
                                                    )
                                                }
                                            >
                                                {disabled ? "恢复公司" : "停用公司"}
                                            </Button>
                                        </div>
                                    );
                                },
                            },
                        ],
                    }}
                />
            </div>
            <AdminUserDetailDrawer key={detailUserId} userId={detailUserId} onClose={() => setDetailUserId(null)} />
            <AdminModal
                open={createOpen}
                destroyOnHidden
                title="开通公司"
                okText="开通"
                cancelText="取消"
                confirmLoading={saving}
                okButtonProps={{ htmlType: "submit", form: "admin-create-company", disabled: !groups.data || groups.isError }}
                onCancel={() => !saving && setCreateOpen(false)}
            >
                <Form<CompanyFields> id="admin-create-company" layout="vertical" onFinish={create} disabled={saving} initialValues={{ admin: { userGroup: DEFAULT_USER_GROUP_CODE } }}>
                    <Form.Item name="name" label="公司名称" rules={[{ required: true, whitespace: true, message: "请填写公司名称" }]}>
                        <Input maxLength={120} />
                    </Form.Item>
                    <Form.Item name={["admin", "username"]} label="首位管理员用户名" rules={[{ required: true, whitespace: true, message: "请填写管理员用户名" }]}>
                        <Input autoComplete="off" />
                    </Form.Item>
                    <Form.Item name={["admin", "email"]} label="首位管理员邮箱" rules={[{ type: "email", message: "请输入有效邮箱" }]}>
                        <Input type="email" autoComplete="off" />
                    </Form.Item>
                    <Form.Item name={["admin", "password"]} label="首位管理员初始密码" rules={[{ required: true, min: 8, message: "请输入至少 8 位密码" }]}>
                        <Input.Password autoComplete="new-password" />
                    </Form.Item>
                    {groups.isError && <Alert type="error" title="用户分组加载失败" action={<Button onClick={() => void groups.refetch()}>重试</Button>} />}
                    <Form.Item name={["admin", "userGroup"]} label="管理员用户分组" extra="请选择已获得固定渠道授权的用户分组。" rules={[{ required: true, message: "请选择用户分组" }]}>
                        <Select appearance="field" loading={groups.isFetching} options={(groups.data?.groups ?? []).map((group) => ({ value: group.code, label: group.name }))} />
                    </Form.Item>
                </Form>
            </AdminModal>
            <AdminModal
                open={Boolean(addingTo)}
                destroyOnHidden
                title={`添加管理员 · ${addingTo?.name ?? ""}`}
                okText="添加"
                cancelText="取消"
                confirmLoading={saving}
                okButtonProps={{ htmlType: "submit", form: "admin-add-company-admin" }}
                onCancel={() => !saving && setAddingTo(null)}
            >
                <Form id="admin-add-company-admin" layout="vertical" onFinish={addAdmin} disabled={saving}>
                    <Form.Item name="usernameOrEmail" label="现有用户的用户名或邮箱" extra="可选择本公司已启用的员工，或尚未加入任何公司的已启用普通账号。" rules={[{ required: true, whitespace: true, message: "请输入用户名或邮箱" }]}>
                        <Input autoComplete="off" />
                    </Form.Item>
                </Form>
            </AdminModal>
        </AdminPageFrame>
    );
}
