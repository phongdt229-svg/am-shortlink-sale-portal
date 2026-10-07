// Chiều nhóm / tiêu chí lọc của Click Explorer.

export const DIM_GROUPS: { label: string; dims: { key: string; label: string }[] }[] = [
  {
    label: "Đối tượng",
    dims: [
      { key: "account", label: "Tài khoản" },
      { key: "campaign", label: "Chiến dịch" },
      { key: "ctv", label: "CTV" },
      { key: "link", label: "Link" },
    ],
  },
  {
    label: "Thời gian",
    dims: [
      { key: "day", label: "Ngày" },
      { key: "week", label: "Tuần" },
      { key: "month", label: "Tháng" },
      { key: "hour", label: "Giờ trong ngày" },
      { key: "weekday", label: "Thứ trong tuần" },
    ],
  },
  {
    label: "Link",
    dims: [
      { key: "link_prefix", label: "Prefix cấp" },
      { key: "access_prefix", label: "Prefix truy cập" },
      { key: "api_version", label: "Tạo qua API" },
      { key: "dest_host", label: "Domain đích" },
    ],
  },
  {
    label: "Thiết bị & nguồn",
    dims: [
      { key: "device", label: "Thiết bị" },
      { key: "os", label: "Hệ điều hành" },
      { key: "browser", label: "Trình duyệt" },
      { key: "source_group", label: "Nhóm nguồn" },
      { key: "referer_host", label: "Referer" },
      { key: "country", label: "Quốc gia" },
      { key: "province", label: "Tỉnh / thành" },
    ],
  },
];

export const DIM_LABEL: Record<string, string> = Object.fromEntries(DIM_GROUPS.flatMap((g) => g.dims.map((d) => [d.key, d.label])));

export function dimLabel(key: string): string {
  if (key.startsWith("param.")) return `Tham số ${key.slice(6)}`;
  return DIM_LABEL[key] ?? key;
}

export const TIME_DIMS = new Set(["day", "week", "month"]);

/** Tiêu chí lọc dạng chọn nhiều có facets (trường ClickFilters → field của API facets). */
export const FACET_CRITERIA: { key: string; label: string; field: string }[] = [
  { key: "device", label: "Thiết bị", field: "device" },
  { key: "os", label: "Hệ điều hành", field: "os" },
  { key: "browser", label: "Trình duyệt", field: "browser" },
  { key: "source_group", label: "Nhóm nguồn", field: "source_group" },
  { key: "referer_host", label: "Referer", field: "referer_host" },
  { key: "country", label: "Quốc gia", field: "country" },
  { key: "province", label: "Tỉnh / thành", field: "province" },
  { key: "dest_host", label: "Domain đích", field: "dest_host" },
  { key: "access_prefix", label: "Prefix truy cập", field: "access_prefix" },
  { key: "api_version", label: "Tạo qua API", field: "api_version" },
];

export const WEEKDAYS = ["CN", "T2", "T3", "T4", "T5", "T6", "T7"];
