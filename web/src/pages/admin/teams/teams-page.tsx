import { useQuery } from "@tanstack/react-query";
import { Alert, Button } from "antd";
import { useState } from "react";

import { listAdminTeamMembers, listAdminTeams, type AdminTeam, type AdminTeamMember } from "@/services/api/teams";
import { useUserStore } from "@/stores/use-user-store";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminDataTable, AdminStatusBadge, AdminTableEmpty } from "../components/admin-ui";
import { AdminModal } from "../ui/overlays";

export default function AdminTeamsPage() {
    const userId = useUserStore((state) => state.user?.id);
    const [selected, setSelected] = useState<AdminTeam | null>(null);
    const teams = useQuery({ queryKey: ["admin-teams", userId], queryFn: ({ signal }) => listAdminTeams(signal), enabled: Boolean(userId) });
    const members = useQuery({
        queryKey: ["admin-team-members", userId, selected?.id],
        queryFn: selected ? ({ signal }) => listAdminTeamMembers(selected.id, signal) : undefined,
        enabled: Boolean(userId && selected),
    });
    const teamNames = new Map(teams.data?.teams.map((team) => [team.id, team.name]));

    return (
        <AdminPageFrame title="团队管理" description="查看团队、所有者和成员。">
            <div className="admin-panel">
                {teams.isError && <Alert type="error" showIcon title={teams.error.message} action={<Button onClick={() => void teams.refetch()}>重试</Button>} />}
                <AdminDataTable<AdminTeam>
                    skeletonColumns={5}
                    empty={<AdminTableEmpty title={teams.isError ? "团队列表加载失败" : "暂无团队"} />}
                    table={{
                        className: "app-data-table",
                        size: "small",
                        rowKey: "id",
                        loading: teams.isFetching,
                        dataSource: teams.data?.teams ?? [],
                        scroll: { x: 700 },
                        pagination: { pageSize: 20, hideOnSinglePage: true },
                        columns: [
                            { title: "团队", dataIndex: "name" },
                            { title: "上级团队", key: "parent", render: (_, team) => (team.parentId ? teamNames.get(team.parentId) || team.parentId : "—") },
                            { title: "所有者", key: "owner", render: (_, team) => team.ownerDisplayName || team.ownerUsername || team.ownerUserId },
                            { title: "状态", key: "status", render: (_, team) => <TeamStatus status={team.status} /> },
                            {
                                title: "操作",
                                key: "actions",
                                render: (_, team) => (
                                    <Button type="link" aria-label={`查看 ${team.name} 的成员`} onClick={() => setSelected(team)}>
                                        查看成员
                                    </Button>
                                ),
                            },
                        ],
                    }}
                />
            </div>
            <AdminModal open={Boolean(selected)} title={selected ? `${selected.name} · 成员` : "团队成员"} onCancel={() => setSelected(null)} footer={null} width={680}>
                {members.isError && <Alert type="error" showIcon title={members.error.message} action={<Button onClick={() => void members.refetch()}>重试</Button>} />}
                <AdminDataTable<AdminTeamMember>
                    skeletonColumns={4}
                    empty={<AdminTableEmpty title={members.isError ? "成员列表加载失败" : "暂无成员"} />}
                    table={{
                        className: "app-data-table",
                        size: "small",
                        rowKey: "id",
                        loading: members.isFetching,
                        dataSource: members.data?.members ?? [],
                        scroll: { x: 480 },
                        pagination: { pageSize: 12, hideOnSinglePage: true },
                        columns: [
                            { title: "用户名", dataIndex: "username" },
                            { title: "显示名称", dataIndex: "displayName" },
                            { title: "角色", key: "role", render: (_, member) => (member.role === "owner" ? "所有者" : "成员") },
                            { title: "状态", key: "status", render: (_, member) => <TeamStatus status={member.status} /> },
                        ],
                    }}
                />
            </AdminModal>
        </AdminPageFrame>
    );
}

function TeamStatus({ status }: { status: string }) {
    return <AdminStatusBadge label={status === "active" ? "正常" : status === "disabled" ? "停用" : status} tone={status === "active" ? "success" : "neutral"} />;
}
