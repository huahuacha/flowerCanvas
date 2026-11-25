import type { AppLocale } from "./locales";

type Messages = { skin: string; avatar: string; site: string; classic: string; whale: string; orb: string };

export const personalAppearanceMessages: Record<AppLocale, Messages> = {
    "zh-CN": { skin: "界面主题", avatar: "助手形象", site: "跟随站点", classic: "经典", whale: "鲸鱼娘", orb: "动态球" },
    "en-US": { skin: "Interface theme", avatar: "Assistant appearance", site: "Site default", classic: "Classic", whale: "Whale girl", orb: "Animated orb" },
};
