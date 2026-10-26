"use client";

import type {
  DependencyGraph,
  DependencyNode,
  Finding,
} from "@/generated/report-types";
import { SeverityTag } from "@/components/report/SeverityTag";
import { findingsForNode } from "@/lib/format";

type TreeItem = {
  node: DependencyNode;
  children: TreeItem[];
};

const MAX_DEPTH = 8;
const MAX_NODES = 100;

function buildTree(graph: DependencyGraph): {
  root: TreeItem | null;
  truncated: boolean;
} {
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  const rootNode = byId.get(graph.rootNodeId);
  if (!rootNode) {
    return { root: null, truncated: false };
  }

  const visited = new Set<string>();
  let count = 0;
  let truncated = false;

  function walk(node: DependencyNode, depth: number): TreeItem {
    visited.add(node.id);
    count += 1;
    const children: TreeItem[] = [];
    if (depth >= MAX_DEPTH || count >= MAX_NODES) {
      truncated = true;
      return { node, children };
    }
    for (const id of node.dependencies) {
      const child = byId.get(id);
      if (!child) {
        truncated = true;
        continue;
      }
      if (visited.has(child.id)) {
        truncated = true;
        continue;
      }
      children.push(walk(child, depth + 1));
    }
    return { node, children };
  }

  return { root: walk(rootNode, 0), truncated };
}

function pathLabel(
  node: DependencyNode,
  graph: DependencyGraph,
): string | null {
  if (node.relationship === "root" || node.dependencyPath.length < 2) {
    return null;
  }
  const byId = new Map(graph.nodes.map((candidate) => [candidate.id, candidate]));
  return node.dependencyPath
    .map((id) => byId.get(id)?.package.name ?? id)
    .join(" › ");
}

function NodeRecord({
  node,
  graph,
  findings,
  anchorFor,
  openIds,
  onOpenFinding,
  descendants,
}: {
  node: DependencyNode;
  graph: DependencyGraph;
  findings: readonly Finding[];
  anchorFor: ReadonlyMap<string, string>;
  openIds: ReadonlySet<string>;
  onOpenFinding: (id: string) => void;
  descendants: TreeItem[];
}) {
  const nodeFindings = findingsForNode(node, findings);
  const path = pathLabel(node, graph);

  return (
    <li className={`load-node ${node.relationship === "root" ? "load-root" : ""}`}>
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="rule-label text-ink-soft">
          {node.relationship}
        </span>
        <span className="data-value text-sm font-semibold break-all text-ink">
          {node.package.name}
        </span>
        <span className="data-value text-xs text-ink-soft">
          {node.version ?? "unresolved"}
        </span>
        {nodeFindings.length > 0 ? (
          <span className="rule-label text-safety-deep">
            {nodeFindings.length} flagged
          </span>
        ) : null}
      </div>

      {path ? (
        <p className="mt-1 font-mono text-[0.6875rem] leading-4 text-ink-soft">
          {path}
        </p>
      ) : null}

      {nodeFindings.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-2">
          {nodeFindings.map((finding) => {
            const anchor = anchorFor.get(finding.id);
            const open = openIds.has(finding.id);
            return (
              <li key={`${node.id}-${finding.id}`}>
                <button
                  type="button"
                  aria-expanded={open}
                  aria-controls={anchor}
                  onClick={() => onOpenFinding(finding.id)}
                  className={`flex items-center gap-2 border px-2 py-1 text-left text-[0.6875rem] transition-colors ${
                    open
                      ? "border-rule-strong bg-paper-deep"
                      : "border-safety-deep bg-transparent hover:bg-paper-deep"
                  }`}
                >
                  <SeverityTag severity={finding.severity} />
                  <span className="font-mono tracking-[0.06em] text-ink uppercase">
                    {finding.kind === "confirmed" ? "Confirmed" : "Heuristic"}
                  </span>
                  <span className="font-mono tracking-[0.06em] text-ink-soft uppercase">
                    {open ? "Evidence open" : "Open evidence"}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}

      {descendants.length > 0 ? (
        <ol className="mt-4 space-y-4">
          {descendants.map((child) => (
            <NodeRecord
              key={child.node.id}
              node={child.node}
              graph={graph}
              findings={findings}
              anchorFor={anchorFor}
              openIds={openIds}
              onOpenFinding={onOpenFinding}
              descendants={child.children}
            />
          ))}
        </ol>
      ) : null}
    </li>
  );
}

export function LoadPath({
  graph,
  findings,
  anchorFor,
  openIds,
  onOpenFinding,
  idPrefix = "",
}: {
  graph: DependencyGraph;
  findings: readonly Finding[];
  anchorFor: ReadonlyMap<string, string>;
  openIds: ReadonlySet<string>;
  onOpenFinding: (id: string) => void;
  idPrefix?: string;
}) {
  const { root, truncated } = buildTree(graph);
  const headingId = `${idPrefix}load-path-heading`;

  return (
    <section aria-labelledby={headingId}>
      <div className="border-b border-rule pb-3">
        <h2 id={headingId} className="text-xl font-bold tracking-[-0.01em]">
          Dependency load path
        </h2>
        <p className="mt-1 text-sm text-ink-soft">
          Follow the resolved graph from the root to each flagged package. Open a
          marker to read the finding without leaving the path.
        </p>
      </div>

      {root ? (
        <ol className="mt-4">
          <NodeRecord
            node={root.node}
            graph={graph}
            findings={findings}
            anchorFor={anchorFor}
            openIds={openIds}
            onOpenFinding={onOpenFinding}
            descendants={root.children}
          />
        </ol>
      ) : (
        <p className="mt-3 border border-rule bg-paper-deep px-3 py-2 text-sm text-ink-soft">
          The dependency graph does not contain its declared root node. The
          report is shown without a load path.
        </p>
      )}

      {truncated ? (
        <p className="mt-4 border-t border-rule pt-2 text-xs leading-5 text-ink-soft">
          This diagram omits repeated, missing, or deeply nested nodes for
          display. The underlying report is unchanged.
        </p>
      ) : null}
    </section>
  );
}
