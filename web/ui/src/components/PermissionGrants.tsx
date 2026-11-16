import { useEffect, useId, useState, type FormEvent, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ApiError, getPermissions, savePermissions, type Denial, type Detail, type Permissions, type PermissionScope, type RefusedRule } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { defineMessages, errorText, useMessages } from "@/lib/i18n";

/** messages is the text of the refused operations of a wait, and of the granted rules (add-permission-grants). */
const messages = defineMessages({
  ja: {
    denied: "拒否された操作",
    deniedHint: "許可する規則を確かめて「許可して再実行」を押すと、規則を保存してから再実行を頼みます。",
    ruleLabel: (command: string) => `${command} を許可する規則`,
    use: "許可する",
    compound: "つないだコマンドなので、規則では許可できません。直すなら、コマンドを分けて実行するよう返答で伝えてください。",
    noRule: "この操作には、Web表示から足せる規則がありません。",
    scope: "許可する範囲",
    scopes: { issue: "このissue", project: "このプロジェクト", global: "全プロジェクト" },
    grantAndRetry: "許可して再実行",
    grantOnly: "許可だけ",
    saving: "保存しています",
    granted: "許可しました",
    savedNotSent: "許可は保存しましたが、再実行を頼めませんでした",
    refused: (r: RefusedRule) => `${r.rule}：${r.reason}`,
    failed: (detail: string) => `許可を保存できませんでした：${detail}`,
    issueGrants: "このissueの許可",
    remove: (rule: string) => `${rule} を外す`,
    removeShort: "外す",
    heading: "許可",
    host: (host: ReactNode): ReactNode => (
      <>端末 {host} の許可の規則です。この端末のエージェントの、ファイルを書く実行（実装と直す実行）にだけ渡ります。</>
    ),
    global: "全プロジェクト",
    none: "規則はありません。",
    addLabel: (target: string) => `${target} に足す規則`,
    add: (target: string) => `${target} に足す`,
    addShort: "足す",
    readFailed: (detail: string) => `許可を読めませんでした：${detail}`,
  },
  en: {
    denied: "Refused operations",
    deniedHint: "Check the rules to allow, then press “Allow and run again” to save them and ask for another run.",
    ruleLabel: (command: string) => `Rule that allows ${command}`,
    use: "Allow",
    compound: "No rule can allow a command joined with others. To fix it, reply asking the agent to run the commands one by one.",
    noRule: "The web inbox has no rule it may add for this operation.",
    scope: "Allow for",
    scopes: { issue: "This issue", project: "This project", global: "Every project" },
    grantAndRetry: "Allow and run again",
    grantOnly: "Allow only",
    saving: "Saving",
    granted: "Allowed",
    savedNotSent: "The rules were saved, but the request to run again failed",
    refused: (r: RefusedRule) => `${r.rule}: ${r.reason}`,
    failed: (detail: string) => `Could not save the rules: ${detail}`,
    issueGrants: "Rules of this issue",
    remove: (rule: string) => `Remove ${rule}`,
    removeShort: "Remove",
    heading: "Allowed operations",
    host: (host: ReactNode): ReactNode => (
      <>The rules of machine {host}. Only that machine’s agent gives them to the runs that write files (implementing and fixing).</>
    ),
    global: "Every project",
    none: "No rules.",
    addLabel: (target: string) => `Rule to add to ${target}`,
    add: (target: string) => `Add to ${target}`,
    addShort: "Add",
    readFailed: (detail: string) => `Could not read the rules: ${detail}`,
  },
});

/** useCanGrant tells whether this person may read and change the granted rules: the login is off, or an admin. */
function useCanGrant() {
  const { status, known } = useAuth();
  return known && (!status.enabled || !!status.admin);
}

/** projectOf is the project of an issue reference: gh:o/r for gh:o/r#12. */
const projectOf = (ref: string) => ref.replace(/#\d+$/, "");

/** wildcard is the candidate with arguments, which a field shows first: Bash(git mv *) of Bash(git mv) and Bash(git mv *). */
const wildcard = (d: Denial) => d.candidates.find((c) => c.endsWith(" *)")) ?? d.candidates[0] ?? "";

/** Choice is what a person chose for one refused operation: whether to allow it, and the rule, as edited. */
type Choice = { use: boolean; rule: string };

/** rulesOf are the rules the choices save: both candidates of a field left as offered, else the edited rule alone. */
function rulesOf(denials: Denial[], choices: Choice[]): string[] {
  const out: string[] = [];
  denials.forEach((d, i) => {
    const c = choices[i];
    if (!c?.use || c.rule.trim() === "") return;
    const add = c.rule.trim() === wildcard(d) ? d.candidates : [c.rule.trim()];
    for (const r of add) if (!out.includes(r)) out.push(r);
  });
  return out;
}

/**
 * DenialsBox lists, on a wait for a permission, the operations the latest run was refused. To an admin, or with the
 * login off, it offers each candidate rule in a field to edit, the scope to allow it for (this issue by default), and
 * two buttons: “Allow and run again” saves the rules, then sends retry (the reply that runs the agent again);
 * “Allow only” saves them alone. A command joined with others gets no field: no rule matches it.
 */
export function DenialsBox({ detail, retry }: { detail: Detail; retry: () => Promise<boolean> }) {
  const t = useMessages(messages);
  const can = useCanGrant();
  const id = useId();
  const denials = detail.denials ?? [];
  const [choices, setChoices] = useState<Choice[]>(() => denials.map((d) => ({ use: d.candidates.length > 0, rule: wildcard(d) })));
  const [scope, setScope] = useState<PermissionScope["scope"]>("issue");
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; text: string; refused?: RefusedRule[] } | null>(null);
  if (denials.length === 0) return null;
  const rules = rulesOf(denials, choices);

  async function grant(andRetry: boolean) {
    setSaving(true);
    setResult(null);
    try {
      const ref = scope === "issue" ? detail.ref : scope === "project" ? projectOf(detail.ref) : undefined;
      await savePermissions({ scope, ...(ref ? { ref } : {}), add: rules });
    } catch (err) {
      const refused = err instanceof ApiError ? err.rules : undefined;
      setResult({ ok: false, text: refused?.length ? "" : t.failed(errorText(err)), refused });
      setSaving(false);
      return;
    }
    if (andRetry && !(await retry())) {
      setResult({ ok: false, text: t.savedNotSent });
    } else if (!andRetry) {
      setResult({ ok: true, text: t.granted });
    }
    setSaving(false);
  }

  return (
    <section aria-labelledby={`${id}-heading`} className="flex flex-col gap-sm">
      <Card className="flex flex-col gap-sm">
        <h2 id={`${id}-heading`} className="text-body-md font-semibold">
          {t.denied}
        </h2>
        {can && <p className="text-label-md text-muted">{t.deniedHint}</p>}
        <ul className="flex flex-col gap-sm">
          {denials.map((d, i) => (
            <li key={i} className="flex flex-col gap-xs">
              <code className="break-all font-mono text-mono-sm">{d.command || d.tool}</code>
              {d.compound ? (
                <p className="text-label-md text-muted">{t.compound}</p>
              ) : d.candidates.length === 0 ? (
                <p className="text-label-md text-muted">{t.noRule}</p>
              ) : (
                can && (
                  <div className="flex flex-wrap items-center gap-sm">
                    <label className="flex min-h-11 items-center gap-xs text-label-md">
                      <input
                        type="checkbox"
                        checked={choices[i]?.use ?? false}
                        onChange={(e) => setChoices((cs) => cs.map((c, j) => (j === i ? { ...c, use: e.target.checked } : c)))}
                        disabled={saving}
                        className="size-5 accent-primary"
                      />
                      {t.use}
                    </label>
                    <Input
                      aria-label={t.ruleLabel(d.command || d.tool)}
                      value={choices[i]?.rule ?? ""}
                      onChange={(e) => setChoices((cs) => cs.map((c, j) => (j === i ? { ...c, rule: e.target.value } : c)))}
                      disabled={saving}
                      className="min-w-0 flex-1 font-mono text-mono-sm"
                    />
                  </div>
                )
              )}
            </li>
          ))}
        </ul>
        {can && (
          <>
            <fieldset className="flex flex-wrap gap-md">
              <legend className="text-label-md font-medium">{t.scope}</legend>
              {(["issue", "project", "global"] as const).map((s) => (
                <label key={s} className="flex min-h-11 items-center gap-xs text-label-md">
                  <input
                    type="radio"
                    name={`${id}-scope`}
                    value={s}
                    checked={scope === s}
                    onChange={() => setScope(s)}
                    disabled={saving}
                    className="size-5 accent-primary"
                  />
                  {t.scopes[s]}
                </label>
              ))}
            </fieldset>
            {result && !result.ok && (
              <Alert>
                {result.refused?.length ? (
                  <ul>
                    {result.refused.map((r) => (
                      <li key={r.rule} className="break-all">
                        {t.refused(r)}
                      </li>
                    ))}
                  </ul>
                ) : (
                  result.text
                )}
              </Alert>
            )}
            {result?.ok && (
              <p role="status" className="text-label-md text-muted">
                {result.text}
              </p>
            )}
            <div className="flex flex-wrap justify-end gap-sm">
              <Button type="button" variant="secondary" disabled={saving || rules.length === 0} onClick={() => void grant(false)}>
                {t.grantOnly}
              </Button>
              <Button type="button" disabled={saving || rules.length === 0 || !detail.waiting} onClick={() => void grant(true)}>
                {saving ? t.saving : t.grantAndRetry}
              </Button>
            </div>
          </>
        )}
      </Card>
    </section>
  );
}

/** IssueGrants lists the rules granted to one issue, each with a button that removes it, to an admin or with the login off. */
export function IssueGrants({ issueRef }: { issueRef: string }) {
  const t = useMessages(messages);
  const can = useCanGrant();
  const id = useId();
  const [rules, setRules] = useState<string[]>([]);
  useEffect(() => {
    if (!can) return;
    let live = true;
    getPermissions(issueRef).then(
      (p) => live && setRules(p.items.find((it) => it.scope === "issue")?.rules ?? []),
      () => live && setRules([]),
    );
    return () => {
      live = false;
    };
  }, [can, issueRef]);
  if (!can || rules.length === 0) return null;
  const remove = (rule: string) =>
    savePermissions({ scope: "issue", ref: issueRef, remove: [rule] }).then(
      () => setRules((rs) => rs.filter((r) => r !== rule)),
      () => {},
    );
  return (
    <section aria-labelledby={`${id}-grants`} className="flex flex-col gap-xs">
      <h2 id={`${id}-grants`} className="text-label-md font-medium">
        {t.issueGrants}
      </h2>
      <RuleList rules={rules} onRemove={remove} />
    </section>
  );
}

/** RuleList lists rules as code, each with a button that removes it. */
function RuleList({ rules, onRemove }: { rules: string[]; onRemove: (rule: string) => void }) {
  const t = useMessages(messages);
  if (rules.length === 0) return <p className="text-label-md text-muted">{t.none}</p>;
  return (
    <ul className="flex flex-col">
      {rules.map((r) => (
        <li key={r} className="flex min-h-11 items-center justify-between gap-sm">
          <code className="break-all font-mono text-mono-sm">{r}</code>
          <Button type="button" variant="secondary" aria-label={t.remove(r)} onClick={() => onRemove(r)}>
            {t.removeShort}
          </Button>
        </li>
      ))}
    </ul>
  );
}

/**
 * PermissionsSection is the part of the settings with the rules granted to every project and to each project: a list
 * with a button that removes each rule, and a field that adds one. It names the machine whose agent alone reads them.
 */
export function PermissionsSection() {
  const t = useMessages(messages);
  const [data, setData] = useState<Permissions | null>(null);
  const [error, setError] = useState<string | null>(null);
  // An inbox service older than the permissions, or one not reachable (the instructions say so already), hides the section.
  const [hidden, setHidden] = useState(false);
  const load = () =>
    getPermissions().then(
      (p) => {
        setData(p);
        setError(null);
        setHidden(false);
      },
      (err) => {
        if (err instanceof ApiError && (err.code === "not_found" || err.code === "unavailable")) setHidden(true);
        else setError(t.readFailed(errorText(err)));
      },
    );
  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const change = async (req: Parameters<typeof savePermissions>[0]) => {
    try {
      await savePermissions(req);
      setError(null);
      await load();
      return true;
    } catch (err) {
      const refused = err instanceof ApiError ? err.rules : undefined;
      setError(refused?.length ? refused.map(t.refused).join(" / ") : t.failed(errorText(err)));
      return false;
    }
  };
  if (hidden) return null;
  return (
    <section aria-labelledby="permissions-heading" className="flex flex-col gap-sm">
      <h2 id="permissions-heading" className="text-body-md font-semibold">
        {t.heading}
      </h2>
      {data && <p className="text-label-md text-muted">{t.host(<span className="font-mono text-mono-sm">{data.host}</span>)}</p>}
      {error && <Alert>{error}</Alert>}
      {data?.items
        .filter((it) => it.scope !== "issue")
        .map((it) => {
          const target = it.scope === "global" ? t.global : (it.ref ?? "");
          const base = it.scope === "global" ? { scope: "global" as const } : { scope: "project" as const, ref: it.ref };
          return (
            <Card key={`${it.scope}-${it.ref ?? ""}`} className="flex flex-col gap-xs">
              <h3 className="text-label-md font-medium">{target}</h3>
              <RuleList rules={it.rules} onRemove={(r) => void change({ ...base, remove: [r] })} />
              <AddRule target={target} onAdd={(r) => change({ ...base, add: [r] })} />
            </Card>
          );
        })}
    </section>
  );
}

/** AddRule is the field and the button that add one rule to target. */
function AddRule({ target, onAdd }: { target: string; onAdd: (rule: string) => Promise<boolean> }) {
  const t = useMessages(messages);
  const [rule, setRule] = useState("");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (rule.trim() === "") return;
    if (await onAdd(rule.trim())) setRule("");
  };
  return (
    <form onSubmit={(e) => void submit(e)} className="flex flex-wrap items-center gap-sm">
      <Input aria-label={t.addLabel(target)} value={rule} onChange={(e) => setRule(e.target.value)} className="min-w-0 flex-1 font-mono text-mono-sm" />
      <Button type="submit" variant="secondary" aria-label={t.add(target)} disabled={rule.trim() === ""}>
        {t.addShort}
      </Button>
    </form>
  );
}
