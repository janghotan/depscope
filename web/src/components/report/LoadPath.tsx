"use client";

import { useState } from "react";

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
  selectedNodeId,
  onSelectNode,
  descendants,
}: {
  node: DependencyNode;
  graph: DependencyGraph;
  findings: readonly Finding[];
  anchorFor: ReadonlyMap<string, string>;
  openIds: ReadonlySet<string>;
  onOpenFinding: (id: string) => void;
  selectedNodeId: string | null;
  onSelectNode: (id: string | null) => void;
  descendants: TreeItem[];
}) {
  const nodeFindings = findingsForNode(node, findings);
  const path = pathLabel(node, graph);
  const selected = selectedNodeId === node.id;
  const detailsId = `load-node-details-${encodeURIComponent(node.id)}`;

  return (
    <li className={`load-node ${node.relationship === "root" ? "load-root" : ""}`}>
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="rule-label text-ink-soft">
          {node.relationship}
        </span>
        <button
          type="button"
          aria-label={`Inspect package ${node.package.name}`}
          aria-expanded={selected}
          aria-controls={detailsId}
          onClick={() => onSelectNode(selected ? null : node.id)}
          className="data-value text-left text-sm font-semibold break-all text-ink underline decoration-rule-strong underline-offset-4 hover:decoration-ink"
        >
          <span className="sr-only">Inspect package </span>
          {node.package.name}
        </button>
        <span className="data-value text-xs text-ink-soft">
          {node.version ? `@ ${node.version}` : "version unresolved"}
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

      <div
        id={detailsId}
        role="region"
        aria-label={`Package details for ${node.package.name}`}
        hidden={!selected}
        className="mt-3 border border-rule bg-paper-deep/60 p-3"
      >
          <dl className="grid gap-x-5 gap-y-2 text-xs sm:grid-cols-2">
            <div>
              <dt className="rule-label text-ink-soft">Package</dt>
              <dd className="data-value mt-1 break-all text-ink">
                {node.package.ecosystem} · {node.package.name}
              </dd>
            </div>
            <div>
              <dt className="rule-label text-ink-soft">Relationship</dt>
              <dd className="mt-1 text-ink">{node.relationship}</dd>
            </div>
            <div>
              <dt className="rule-label text-ink-soft">Resolved version</dt>
              <dd className="data-value mt-1 text-ink">
                {node.version ?? "unresolved"}
              </dd>
            </div>
            <div>
              <dt className="rule-label text-ink-soft">Dependency path</dt>
              <dd className="mt-1 break-words text-ink">
                {path ?? node.package.name}
              </dd>
            </div>
          </dl>
          <div className="mt-3 border-t border-rule pt-3">
            <p className="rule-label text-ink-soft">Associated evidence</p>
            {nodeFindings.length > 0 ? (
              <ul className="mt-2 space-y-2">
                {nodeFindings.map((finding) => (
                  <li key={`detail-${node.id}-${finding.id}`}>
                    <button
                      type="button"
                      className="link-quiet text-left text-sm font-semibold text-ink"
                      aria-label={`Open evidence for ${finding.title}`}
                      onClick={() => onOpenFinding(finding.id)}
                    >
                      {finding.kind === "confirmed"
                        ? "Confirmed · "
                        : "Heuristic · "}
                      {finding.title}
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-1 text-sm text-ink-soft">
                No finding is associated with this package in the report.
              </p>
            )}
          </div>
      </div>

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
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
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
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
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
            selectedNodeId={selectedNodeId}
            onSelectNode={setSelectedNodeId}
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
