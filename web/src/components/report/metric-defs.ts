// Tooltip giải thích chỉ số — đúng định nghĩa PLAN_SERVICE_API.md §5.2 (portal-api tính, frontend chỉ hiển thị).
export const METRIC_DEFS = {
  total_links: { label: "Tổng link", hint: "Số link chưa xoá thuộc phạm vi lọc, tạo đến hết kỳ" },
  new_links: { label: "Link mới", hint: "Số link tạo trong kỳ" },
  active_links: { label: "Link có click", hint: "Số link có ≥ 1 click (không tính bot) trong kỳ" },
  clicks: { label: "Lượt click", hint: "Tổng lượt redirect thành công trong kỳ, không tính bot" },
  unique_clicks: { label: "Khách duy nhất", hint: "Số khách duy nhất / link / ngày (khách = IP + user-agent), cộng dồn theo ngày" },
  bot_clicks: { label: "Click bot", hint: "Lượt click nhận là bot — tách riêng, không cộng vào lượt click" },
  suspicious_clicks: { label: "Click nghi vấn", hint: "Click từ IP vượt ngưỡng (> 20 click / link / giờ) — dùng soát gian lận" },
  ctr_per_link: { label: "Click / link", hint: "Lượt click chia số link có click" },
} as const;

export type MetricKey = keyof typeof METRIC_DEFS;

export const BREAKDOWN_LABELS: Record<string, string> = {
  device: "Thiết bị",
  os: "Hệ điều hành",
  browser: "Trình duyệt",
  source_group: "Nhóm nguồn",
  referer: "Referer",
  country: "Quốc gia",
  access_prefix: "Prefix truy cập",
};

export const SOURCE_LABELS: Record<string, string> = {
  zalo: "Zalo",
  facebook: "Facebook",
  google: "Google",
  tiktok: "TikTok",
  direct: "Trực tiếp",
  other: "Khác",
};
