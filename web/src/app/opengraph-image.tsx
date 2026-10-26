import { ImageResponse } from "next/og";

export const alt = "DepScope — dependency condition records";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          width: "100%",
          height: "100%",
          padding: "68px 76px",
          color: "#e5eceb",
          background: "#0d1519",
          fontFamily: "sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 18 }}>
          <div
            style={{
              width: 22,
              height: 22,
              border: "5px solid #f1855c",
              borderRadius: 99,
            }}
          />
          <div style={{ fontSize: 26, fontWeight: 700, letterSpacing: 5 }}>
            DEPSCOPE
          </div>
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
          <div
            style={{
              color: "#b8c5c8",
              fontSize: 22,
              letterSpacing: 4,
              textTransform: "uppercase",
            }}
          >
            Package inspection · npm / PyPI
          </div>
          <div
            style={{
              maxWidth: 940,
              fontSize: 76,
              fontWeight: 700,
              lineHeight: 1.04,
              letterSpacing: -2,
            }}
          >
            Evidence before you add a dependency.
          </div>
          <div style={{ color: "#b8c5c8", fontSize: 28 }}>
            Advisories · heuristics · coverage gaps
          </div>
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            color: "#f0c46a",
            fontSize: 18,
            letterSpacing: 3,
            textTransform: "uppercase",
          }}
        >
          Shared Go security engine · schema v1.0.0
        </div>
      </div>
    ),
    size,
  );
}
