import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Input, Spin, Tag } from "antd";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { currentLocale } from "@/i18n";
import { deleteFixedChannelCredential, listFixedChannels, saveFixedChannelCredential, type FixedChannel } from "@/services/api/fixed-channels";
import { useUserStore } from "@/stores/use-user-store";

export function FixedChannelsPane() {
    const { t } = useTranslation();
    const userId = useUserStore((state) => state.user?.id);
    const queryClient = useQueryClient();
    const queryKey = ["fixed-channels", userId];
    const channels = useQuery({ queryKey, queryFn: ({ signal }) => listFixedChannels(signal), enabled: Boolean(userId) });
    const updateChannel = (channel: FixedChannel) => {
        queryClient.setQueryData<{ channels: FixedChannel[] }>(queryKey, (data) => data && { channels: data.channels.map((item) => (item.id === channel.id ? channel : item)) });
        void channels.refetch();
    };

    return (
        <>
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>{t("company.fixed-channels.label")}</h2>
                    <p>{t("company.fixedChannels.description")}</p>
                </div>
            </div>
            <div className="settings-section space-y-4">
                {channels.isPending && <Spin />}
                {channels.isError && <Alert type="error" showIcon title={channels.error.message} action={<Button onClick={() => void channels.refetch()}>{t("common.retry")}</Button>} />}
                {channels.data?.channels.length === 0 && <p className="text-sm text-muted-foreground">{t("company.fixedChannels.empty")}</p>}
                {channels.data?.channels.map((channel) => (
                    <FixedChannelCredential key={channel.id} channel={channel} onChanged={updateChannel} />
                ))}
            </div>
        </>
    );
}

function FixedChannelCredential({ channel, onChanged }: { channel: FixedChannel; onChanged: (channel: FixedChannel) => void }) {
    const { t } = useTranslation();
    const { message, modal } = App.useApp();
    const [draft, setDraft] = useState("");
    const [working, setWorking] = useState<"save" | "remove" | null>(null);
    const submitting = useRef(false);

    const save = async () => {
        if (submitting.current || !draft.trim()) return;
        submitting.current = true;
        setWorking("save");
        try {
            const saved = await saveFixedChannelCredential(channel.id, draft.trim());
            setDraft("");
            onChanged(saved);
            message.success(t("company.fixedChannels.saved"));
        } catch (error) {
            message.error(error instanceof Error ? error.message : t("company.fixedChannels.saveFailed"));
        } finally {
            submitting.current = false;
            setWorking(null);
        }
    };

    const remove = () => {
        modal.confirm({
            title: t("company.fixedChannels.removeTitle", { name: channel.name }),
            content: t("company.fixedChannels.removeDescription"),
            okText: t("company.fixedChannels.remove"),
            cancelText: t("common.cancel"),
            okButtonProps: { danger: true },
            onOk: async () => {
                if (submitting.current) return;
                submitting.current = true;
                setWorking("remove");
                try {
                    await deleteFixedChannelCredential(channel.id);
                    setDraft("");
                    onChanged({ ...channel, hasCredential: false, credentialUpdatedAt: undefined });
                    message.success(t("company.fixedChannels.removed"));
                } catch (error) {
                    message.error(error instanceof Error ? error.message : t("company.fixedChannels.removeFailed"));
                    throw error;
                } finally {
                    submitting.current = false;
                    setWorking(null);
                }
            },
        });
    };

    return (
        <section className="space-y-3 rounded-lg border border-border p-4">
            <div className="flex flex-wrap items-center gap-2">
                <h3 className="font-medium">{channel.name}</h3>
                <Tag color={channel.hasCredential ? "success" : "default"}>{t(channel.hasCredential ? "company.fixedChannels.configured" : "company.fixedChannels.unconfigured")}</Tag>
                {!channel.enabled && <Tag>{t("company.fixedChannels.disabled")}</Tag>}
                {channel.hasCredential && channel.credentialUpdatedAt && (
                    <time dateTime={channel.credentialUpdatedAt} className="text-xs text-muted-foreground">
                        {t("company.fixedChannels.updated", { date: new Date(channel.credentialUpdatedAt).toLocaleString(currentLocale()) })}
                    </time>
                )}
            </div>
            <label className="block text-sm" htmlFor={`fixed-key-${channel.id}`}>
                {t("company.fixedChannels.keyLabel")}
            </label>
            <form
                className="flex flex-wrap gap-2"
                onSubmit={(event) => {
                    event.preventDefault();
                    void save();
                }}
            >
                <Input.Password id={`fixed-key-${channel.id}`} className="min-w-48 flex-1" autoComplete="new-password" value={draft} disabled={Boolean(working)} onChange={(event) => setDraft(event.target.value)} />
                <Button type="primary" htmlType="submit" disabled={!draft.trim() || Boolean(working)} loading={working === "save"}>
                    {t(channel.hasCredential ? "company.fixedChannels.replace" : "company.fixedChannels.save")}
                </Button>
                {channel.hasCredential && (
                    <Button danger disabled={Boolean(working)} loading={working === "remove"} onClick={remove}>
                        {t("company.fixedChannels.remove")}
                    </Button>
                )}
            </form>
        </section>
    );
}
