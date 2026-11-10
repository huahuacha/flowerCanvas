import { useState } from "react";
import { Input, InputNumber, Select } from "antd";
import { parseLumiOptions, setLumiMode, type LumiMode } from "@/lib/lumi-video";

export function LumiVideoSettings({ value, onChange }: { value?: string; onChange: (value: string) => void }) {
    const [error, setError] = useState("");
    let options: Record<string, unknown> = {};
    try {
        options = parseLumiOptions(value);
    } catch {
        /* Surface malformed persisted values without overwriting them. */
    }
    const update = (key: string, next: unknown) => {
        try {
            const current = parseLumiOptions(value);
            if (next === undefined || next === null || next === "") delete current[key];
            else current[key] = next;
            onChange(JSON.stringify(current));
            setError("");
        } catch {
            setError("保存的 Lumi 参数无法读取，请切换制作模式重置。");
        }
    };
    const mode: LumiMode = "draft_task_id" in options ? "upgrade" : options.draft === true ? "draft" : options.omni_reference_task_type === "edit" ? "edit" : options.omni_reference_task_type === "extend" ? "extend" : "normal";
    const field = (label: string, child: React.ReactNode) => (
        <label className="flex min-w-0 flex-col gap-1 text-xs">
            <span>{label}</span>
            {child}
        </label>
    );
    const triState = (key: string) => (
        <Select
            aria-label={key === "generate_audio" ? "Lumi 生成声音" : key === "watermark" ? "Lumi 水印" : "Lumi 返回尾帧"}
            value={options[key] === undefined ? "auto" : String(options[key])}
            options={[...(key === "return_last_frame" ? [{ value: "auto", label: "由模型决定" }] : []), { value: "true", label: "开启" }, { value: "false", label: "关闭" }]}
            onChange={(v) => update(key, v === "auto" ? undefined : v === "true")}
        />
    );
    return (
        <section className="space-y-3" aria-label="Lumi 视频参数" onMouseDown={(e) => e.stopPropagation()}>
            <div className="text-sm font-semibold">Lumi 视频设置</div>
            <p className="text-xs opacity-70">通过平台生成；本版开放普通生成及 2.5 样片。每次生成独立计费。</p>
            {field(
                "制作模式",
                <Select
                    aria-label="Lumi 制作模式"
                    value={mode}
                    options={[
                        { value: "normal", label: "普通生成" },
                        { value: "draft", label: "480p 样片 · 2.5" },
                        { value: "upgrade", label: "升清 · 待接入规格计费", disabled: true },
                        { value: "edit", label: "编辑 · 待接入规格计费", disabled: true },
                        { value: "extend", label: "续写 · 待接入规格计费", disabled: true },
                    ]}
                    onChange={(v) => {
                        onChange(JSON.stringify(setLumiMode(options, v as LumiMode)));
                        setError("");
                    }}
                />,
            )}
            {mode === "upgrade" ? (
                field(
                    "原平台样片任务 ID",
                    <Input
                        aria-label="原平台样片任务 ID"
                        placeholder="task_…（自己的成功样片）"
                        value={String(options.draft_task_id || "")}
                        onChange={(e) => {
                            onChange(JSON.stringify({ draft_task_id: e.target.value, resolution: "1080p" }));
                        }}
                    />,
                )
            ) : (
                <>
                    {field("画幅", <Select aria-label="Lumi 画幅" value={String(options.ratio || "")} options={[...["adaptive", "16:9", "9:16", "1:1", "4:3", "3:4", "21:9"].map((v) => ({ value: v, label: v }))]} onChange={(v) => update("ratio", v)} />)}
                    {mode !== "draft" && field("分辨率", <Select aria-label="Lumi 分辨率" value={String(options.resolution || "")} options={[...["480p", "720p", "1080p"].map((v) => ({ value: v, label: v }))]} onChange={(v) => update("resolution", v)} />)}
                    {field(
                        "时长（秒）",
                        <InputNumber aria-label="Lumi 时长" className="w-full" min={4} max={30} precision={0} value={typeof options.duration === "number" ? options.duration : null} placeholder="4～30 秒" onChange={(v) => update("duration", v)} />,
                    )}
                    <div className="grid grid-cols-2 gap-2">
                        {field("生成声音", triState("generate_audio"))}
                        {field("水印", triState("watermark"))}
                    </div>
                    {field("请求尾帧（平台查询获取）", triState("return_last_frame"))}
                    {field(
                        "随机种子",
                        <InputNumber
                            aria-label="Lumi 随机种子"
                            className="w-full"
                            precision={0}
                            min={Number.MIN_SAFE_INTEGER}
                            max={Number.MAX_SAFE_INTEGER}
                            value={typeof options.seed === "number" ? options.seed : null}
                            onChange={(v) => update("seed", v)}
                        />,
                    )}
                    {field("优先级（0～9）", <InputNumber aria-label="Lumi 优先级" className="w-full" min={0} max={9} precision={0} value={typeof options.priority === "number" ? options.priority : null} onChange={(v) => update("priority", v)} />)}
                    {field(
                        "输出格式",
                        <Select
                            aria-label="Lumi 输出格式"
                            value={String(options.output_format || "")}
                            options={[
                                { value: "", label: "由模型决定" },
                                { value: "mp4", label: "MP4" },
                                { value: "mov", label: "MOV" },
                            ]}
                            onChange={(v) => update("output_format", v)}
                        />,
                    )}
                    {field("平台素材项目标识（可选）", <Input aria-label="平台素材项目标识" value={String(options.project_name || "")} maxLength={64} onChange={(e) => update("project_name", e.target.value)} />)}
                </>
            )}
            <p className="text-xs opacity-70">首帧或首尾帧请选择 adaptive。本版要求明确时长、画幅与分辨率，以保证报价与实际请求一致。</p>
            {error && (
                <p role="alert" className="text-xs text-red-500">
                    {error}
                </p>
            )}
        </section>
    );
}
