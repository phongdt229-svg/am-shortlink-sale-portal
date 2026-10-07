// Role + user MongoDB tối thiểu cho portal-api (docs/PLAN_PORTAL_REPORT.md §3.1).
// Chạy bởi DBA / Ops (mongosh, quyền userAdmin trên admin):
//   mongosh "$ADMIN_URI" --file deploy/mongo/portal_api_role.js
// Mật khẩu lấy từ biến môi trường PORTAL_API_MONGO_PASSWORD (không ghi vào file / git).
//
//   am_shortlink         : CHỈ ĐỌC (users, links, campaigns, prefixes, clicks)
//   am_shortlink_report  : đọc thống kê; GHI chỉ portal_* và param_registry
//   cmd/migrate tạo index portal_* → cần createIndex trên các collection đó.

const portalCollections = [
  "portal_refresh_tokens",
  "portal_login_attempts",
  "portal_saved_reports",
  "portal_exports",
  "portal_audit_log",
  "portal_schema_migrations",
  "param_registry",
];

const admin = db.getSiblingDB("admin");
const roleName = "portalApi";
const privileges = [
  { resource: { db: "am_shortlink", collection: "" }, actions: ["find", "listCollections", "listIndexes"] },
  { resource: { db: "am_shortlink_report", collection: "" }, actions: ["find", "listCollections", "listIndexes"] },
  ...portalCollections.map((c) => ({
    resource: { db: "am_shortlink_report", collection: c },
    actions: ["find", "insert", "update", "remove", "createCollection", "createIndex"],
  })),
];

if (admin.getRole(roleName)) {
  admin.updateRole(roleName, { privileges, roles: [] });
  print(`updated role ${roleName}`);
} else {
  admin.createRole({ role: roleName, privileges, roles: [] });
  print(`created role ${roleName}`);
}

const pwd = process.env.PORTAL_API_MONGO_PASSWORD;
if (!pwd) {
  print("PORTAL_API_MONGO_PASSWORD trống — chỉ tạo / cập nhật role, bỏ qua user");
} else if (admin.getUser("portal_api")) {
  admin.updateUser("portal_api", { pwd, roles: [{ role: roleName, db: "admin" }] });
  print("updated user portal_api");
} else {
  admin.createUser({ user: "portal_api", pwd, roles: [{ role: roleName, db: "admin" }] });
  print("created user portal_api");
}
