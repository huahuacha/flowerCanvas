import { resolveModelRequestConfig, type AiConfig } from "@/stores/use-config-store";
export type LumiOptions = Record<string, unknown>;
export type LumiMode = "normal" | "draft" | "upgrade" | "edit" | "extend";

export function parseLumiOptions(raw?: string): LumiOptions {
    if (!raw?.trim()) return {};
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Lumi 参数必须为对象");
    return value as LumiOptions;
}

export function setLumiMode(_previous: LumiOptions, mode: LumiMode): LumiOptions {
    // Changing the operation clears incompatible fields, rather than silently submitting stale parameters.
    if (mode === "upgrade") return { draft_task_id: "", resolution: "1080p" };
    if (mode === "draft") return { draft: true, resolution: "480p" };
    if (mode === "edit") return { omni_reference_task_type: "edit", ratio: "adaptive", duration: -1 };
    if (mode === "extend") return { omni_reference_task_type: "extend", ratio: "adaptive" };
    return {};
}

export function lumiOptionsForConfig(config: AiConfig): LumiOptions {
    const options: LumiOptions = {
        duration: Number(config.videoSeconds) || 5,
        ratio: config.size || "16:9",
        resolution: config.vquality || "720p",
        generate_audio: config.videoGenerateAudio === "true",
        watermark: config.videoWatermark === "true",
        ...parseLumiOptions(config.lumiVideoParameters),
    };
    const resolution = String(options.resolution).trim().toLowerCase();
    options.resolution = ["480", "720", "1080"].includes(resolution) ? `${resolution}p` : resolution;
    return options;
}
export function projectLumiConfig(config: AiConfig): AiConfig {
    if (resolveModelRequestConfig(config, config.model).interfaceType !== "lumi-ark-video") return config;
    const o = lumiOptionsForConfig(config);
    return { ...config, size: String(o.ratio), vquality: String(o.resolution), videoSeconds: String(o.duration), videoGenerateAudio: String(o.generate_audio), videoWatermark: String(o.watermark), lumiVideoParameters: JSON.stringify(o) };
}
