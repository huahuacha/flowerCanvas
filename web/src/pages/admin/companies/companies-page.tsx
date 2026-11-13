import { useQuery } from "@tanstack/react-query";
import { Alert, App, Button, Form, Input } from "antd";
import { Plus } from "lucide-react";
import { useRef, useState } from "react";

import { createAdminCompany, listAdminCompanies, type AdminCompany } from "@/services/api/company";
import { useUserStore } from "@/stores/use-user-store";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminDataTable, AdminTableEmpty } from "../components/admin-ui";
import { AdminModal } from "../ui/overlays";

type CompanyFields = { name: string; adminUsername: string };

export default function CompaniesPage() {
    const { message } = App.useApp();
    const userId = useUserStore((state) => state.user?.id);
    const companies = useQuery({ queryKey: ["admin-companies", userId], queryFn: ({ signal }) => listAdminCompanies(signal), enabled: Boolean(userId) });
    const [createOpen, setCreateOpen] = useState(false);
    const [saving, setSaving] = useState(false);
    const submitting = useRef(false);

    const create = async ({ name, adminUsername }: CompanyFields) => {
        if (submitting.current) return;
        submitting.current = true;
        setSaving(true);
        try {
            await createAdminCompany({ name: name.trim(), adminUsername: adminUsername.trim() });
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

    return (
        <AdminPageFrame
            title="公司管理"
            description="开通公司并指定首位公司管理员。公司管理员可以开通员工账号并分配积分。"
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
                        scroll: { x: 620 },
                        pagination: { pageSize: 20, hideOnSinglePage: true },
                        columns: [
                            { title: "公司", dataIndex: "name" },
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
                            { title: "成员数", dataIndex: "memberCount", align: "right" },
                            { title: "开通时间", dataIndex: "createdAt", render: (date: string) => new Date(date).toLocaleString() },
                        ],
                    }}
                />
            </div>
            <AdminModal open={createOpen} title="开通公司" okText="开通" cancelText="取消" confirmLoading={saving} okButtonProps={{ htmlType: "submit", form: "admin-create-company" }} onCancel={() => !saving && setCreateOpen(false)}>
                <Form<CompanyFields> id="admin-create-company" layout="vertical" onFinish={create} disabled={saving}>
                    <Form.Item name="name" label="公司名称" rules={[{ required: true, whitespace: true, message: "请填写公司名称" }]}>
                        <Input maxLength={120} />
                    </Form.Item>
                    <Form.Item name="adminUsername" label="公司管理员用户名或邮箱" extra="请选择已启用且尚未加入公司的普通用户。其用户分组需要获得固定渠道授权。" rules={[{ required: true, whitespace: true, message: "请填写管理员用户名或邮箱" }]}>
                        <Input autoComplete="off" maxLength={160} />
                    </Form.Item>
                </Form>
            </AdminModal>
        </AdminPageFrame>
    );
}
