import { VideoSettingsPanel, videoResolutionLabel, videoSecondsLabel, videoSizeLabel } from "@/components/video-settings-panel";
import { canvasThemes } from "@/lib/canvas-theme";
import { projectLumiConfig } from "@/lib/lumi-video";
import { modelCapabilityConfigFor, resolveVideoRatioValue, resolveVideoResolutionValue } from "@/lib/model-capabilities";
import { useActiveTheme } from "@/stores/canvas/use-canvas-theme-store";
import type { AiConfig } from "@/stores/use-config-store";
import { useTranslation } from "react-i18next";
import { CanvasSettingsPopover, type CanvasSettingsPlacement } from "./canvas-settings-popover";

type VideoPopoverProps = { config: AiConfig; onConfigChange: (key: keyof AiConfig, value: string) => void; buttonClassName?: string; placement?: CanvasSettingsPlacement };

export function CanvasVideoSettingsPopover({ config, onConfigChange, ...popover }: VideoPopoverProps) {
    const { t } = useTranslation();
    const theme = canvasThemes[useActiveTheme()];
    const displayConfig = projectLumiConfig(config);
    const profile = modelCapabilityConfigFor(config, config.model).video;
    const parts: string[] = [];
    if (profile?.resolutions.length) parts.push(videoResolutionLabel(resolveVideoResolutionValue(profile, displayConfig.vquality)));
    if (profile?.ratios.length) parts.push(videoSizeLabel(resolveVideoRatioValue(profile, displayConfig.size)));
    parts.push(videoSecondsLabel(displayConfig.videoSeconds));
    return (
        <CanvasSettingsPopover {...popover} label={t("canvasC.videoSettings.label")} summary={parts.join(" · ")}>
            <VideoSettingsPanel config={config} onConfigChange={onConfigChange} onLumiConfigChange={(value) => onConfigChange("lumiVideoParameters", value)} theme={theme} className="space-y-3" />
        </CanvasSettingsPopover>
    );
}
