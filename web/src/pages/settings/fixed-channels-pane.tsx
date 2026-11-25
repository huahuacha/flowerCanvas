import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Input, Select, Spin } from "antd";
import { useId, useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";

import { StatusBadge } from "@/components/ui/base/badges";
import { currentLocale } from "@/i18n";
import { deleteFixedChannelCredential, listFixedChannels, saveFixedChannelCredential, type FixedChannel } from "@/services/api/fixed-channels";
import { adjustPersonalCredits } from "@/services/api/wallet";
import { useUserStore } from "@/stores/use-user-store";

export function FixedChannelsPane() {
    const { t } = useTranslation();
    const userId = useUserStore((state) => state.user?.id);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
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
                    <h2>{t("settings.sections.fixed-channels.label")}</h2>
                    <p>{t("account.settings.fixedChannels.description")}</p>
                </div>
            </div>
            <div className="settings-section space-y-4">
                {channels.isPending && <Spin />}
                {channels.isError && <Alert type="error" showIcon title={channels.error.message} action={<Button onClick={() => void channels.refetch()}>{t("common.retry")}</Button>} />}
                {channels.data?.channels.length === 0 && <p className="text-sm text-muted-foreground">{t("account.settings.fixedChannels.empty")}</p>}
                {channels.data?.channels.map((channel) => (
                    <FixedChannelCredential key={channel.id} channel={channel} onChanged={updateChannel} />
                ))}
                {creditsEnabled && channels.isSuccess && <FixedChannelCreditAdjustment key={userId} channels={channels.data.channels.filter((channel) => channel.enabled && channel.hasCredential)} />}
            </div>
        </>
    );
}

function FixedChannelCreditAdjustment({ channels }: { channels: FixedChannel[] }) {
    const { t } = useTranslation();
    const { message } = App.useApp();
    const channelLabel = useId();
    const [selectedChannel, setSelectedChannel] = useState("");
    const [adjusting, setAdjusting] = useState(false);
    const submitting = useRef(false);
    const channelId = channels.find((channel) => channel.id === selectedChannel)?.id ?? channels[0]?.id;

    const adjustCredits = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!channelId || submitting.current) return;
        const form = event.currentTarget;
        const fields = new FormData(form);
        const parts = /^([+-]?)(\d+)(?:\.(\d{1,6}))?$/.exec(String(fields.get("amount") ?? "").trim());
        const magnitude = parts ? Number(parts[2]) * 1_000_000 + Number((parts[3] ?? "").padEnd(6, "0")) : 0;
        if (!Number.isSafeInteger(magnitude) || magnitude === 0) {
            message.error(t("account.wallet.adjustment.invalid"));
            return;
        }
        const amountMicrocredits = parts?.[1] === "-" ? -magnitude : magnitude;
        const note = String(fields.get("note") ?? "").trim();
        submitting.current = true;
        setAdjusting(true);
        try {
            await adjustPersonalCredits({ channelId, amountMicrocredits, note: note || undefined });
            form.reset();
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success(t("account.wallet.adjustment.success"));
        } catch (error) {
            message.error(error instanceof Error ? error.message : t("account.wallet.adjustment.failed"));
        } finally {
            submitting.current = false;
            setAdjusting(false);
        }
    };

    return (
        <section className="space-y-3 rounded-lg border border-border p-4">
            <h3 className="font-medium">{t("account.wallet.adjustment.title")}</h3>
            {channelId ? (
                <>
                    <p className="text-sm text-muted-foreground">{t("account.wallet.adjustment.description")}</p>
                    <form className="grid max-w-sm gap-3" onSubmit={(event) => void adjustCredits(event)}>
                        <div className="grid gap-1">
                            <span id={channelLabel}>{t("account.wallet.adjustment.channel")}</span>
                            <Select aria-labelledby={channelLabel} value={channelId} disabled={adjusting} onChange={setSelectedChannel} options={channels.map((channel) => ({ value: channel.id, label: channel.name }))} />
                        </div>
                        <label>
                            {t("account.wallet.adjustment.amount")}
                            <Input name="amount" inputMode="decimal" required disabled={adjusting} />
                        </label>
                        <label>
                            {t("account.wallet.adjustment.note")}
                            <Input name="note" maxLength={500} disabled={adjusting} />
                        </label>
                        <Button className="justify-self-start" type="primary" htmlType="submit" loading={adjusting}>
                            {t("account.wallet.adjustment.submit")}
                        </Button>
                    </form>
                </>
            ) : (
                <p className="text-sm text-muted-foreground">{t("account.wallet.adjustment.keyRequired")}</p>
            )}
        </section>
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
            message.success(t("account.settings.fixedChannels.saved"));
        } catch (error) {
            message.error(error instanceof Error ? error.message : t("account.settings.fixedChannels.saveFailed"));
        } finally {
            submitting.current = false;
            setWorking(null);
        }
    };

    const remove = () => {
        modal.confirm({
            title: t("account.settings.fixedChannels.removeTitle", { name: channel.name }),
            content: t("account.settings.fixedChannels.removeDescription"),
            okText: t("account.settings.fixedChannels.remove"),
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
                    message.success(t("account.settings.fixedChannels.removed"));
                } catch (error) {
                    message.error(error instanceof Error ? error.message : t("account.settings.fixedChannels.removeFailed"));
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
                <StatusBadge variant="filled" tone={channel.hasCredential ? "success" : "neutral"} label={t(channel.hasCredential ? "account.settings.fixedChannels.configured" : "account.settings.fixedChannels.unconfigured")} />
                {!channel.enabled && <StatusBadge variant="filled" label={t("account.settings.fixedChannels.disabled")} />}
                {channel.hasCredential && channel.credentialUpdatedAt && (
                    <time dateTime={channel.credentialUpdatedAt} className="text-xs text-muted-foreground">
                        {t("account.settings.fixedChannels.updated", { date: new Date(channel.credentialUpdatedAt).toLocaleString(currentLocale()) })}
                    </time>
                )}
            </div>
            <label className="block text-sm" htmlFor={`fixed-key-${channel.id}`}>
                {t("account.settings.fixedChannels.keyLabel")}
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
                    {t(channel.hasCredential ? "account.settings.fixedChannels.replace" : "account.settings.fixedChannels.save")}
                </Button>
                {channel.hasCredential && (
                    <Button danger disabled={Boolean(working)} loading={working === "remove"} onClick={remove}>
                        {t("account.settings.fixedChannels.remove")}
                    </Button>
                )}
            </form>
        </section>
    );
}
