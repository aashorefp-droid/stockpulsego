"use client";
import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { downloadCsv } from "@/lib/csv";
import { interpretOtmFlow } from "@/lib/optionsFlow";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8000";

const WATCHLISTS = [
  { key: "default",       label: "Default 50",       count: 50  },
  { key: "tech",          label: "Tech 30",           count: 30  },
  { key: "mega_cap",      label: "Mega Cap 20",       count: 20  },
  { key: "momentum",      label: "Momentum 20",       count: 20  },
  { key: "etfs",          label: "ETFs 56",           count: 56  },
  { key: "short_squeeze", label: "🔥 Short Squeeze",  count: 40  },
  { key: "nyse_swing",    label: "🏛 NYSE Swing >$10", count: 200 },
  { key: "nasdaq_swing",  label: "💻 NASDAQ Swing >$10", count: 200 },
  { key: "custom",        label: "Custom",            count: 0   },
];

type Filter = "all" | "actionable" | "rank1" | "exceptional" | "high_short";
type SortBy = "score" | "grade" | "rr" | "swingReward" | "dayReward" | "ltEntryPct" | "valuation";

const SORT_OPTIONS: { key: SortBy; label: string; title?: string }[] = [
  { key: "score",       label: "Score" },
  { key: "grade",       label: "Grade" },
  { key: "rr",          label: "R/R" },
  { key: "swingReward", label: "Swing Reward%", title: "Sort by Swing target reward percent" },
  { key: "dayReward",   label: "Day Reward%",   title: "Sort by Day Trading target reward percent" },
  { key: "ltEntryPct",  label: "LT Entry%",     title: "Sort by Long Term distance from entry" },
  { key: "valuation",   label: "Valuation",     title: "Sort by Long Term valuation estimate" },
];

interface OptLeg {
  action:     string;
  type:       string;
  strike:     number;
  exp:        string;
  bid:        number;
  ask:        number;
  mid:        number;
  spread_pct?: number | null;
}

interface MacroItem {
  ticker:   string;
  label:    string;
  category: string;
  chg_1d:   number;
}

interface ScanResult {
  ticker:        string;
  sector?:       string;
  price?:        number;
  verdict?:      string;
  verdict_flip_date?: string | null;
  verdict_flip_from?: string | null;
  verdict_flip_days?: number | null;
  verdict_flip_text?: string | null;
  confidence?:   string;
  score?:        number;
  direction?:    string;
  entry_grade?:  string;
  entry_label?:  string;
  grade_color?:  string;
  expected_wr?:  number;
  mtf_rank?:     number;
  mtf_signal?:   string;
  mtf_action?:   string;
  mtf_key?:      string;
  weekly_bias?:  string;
  daily_bias?:   string;
  long_term_spring?: boolean;
  long_term_spring_text?: string;
  swing_spring?: boolean;
  swing_spring_text?: string;
  day_spring?: boolean;
  day_spring_text?: string;
  vol_trend?:    string;
  earn_zone?:    string;
  weekly_zone?:  string;
  near_fib_name?: string;
  near_fib_price?: number;
  fib_compression?: boolean;
  signals?:      string;
  valuation_label?: string;
  valuation_score?: number;
  valuation_reason?: string;
  valuation_fair_value?: number | null;
  valuation_upside_pct?: number | null;
  valuation_source?: string;
  cpr_type?:     string;
  cpr_tc?:       number;
  cpr_bc?:       number;
  cpr_p?:        number;
  cpr_position?: string;
  cpr_interpretation?: string;
  cpr_day_result?: string;
  cpr_day_entry?: number | null;
  cpr_day_stop?:  number | null;
  cpr_day_t1?:    number | null;
  cpr_day_trigger_text?: string;
  cpr_day_invalidation_text?: string;
  cpr_day_target_text?: string;
  cpr_day_volume_text?: string;
  cpr_day_15m_volume_text?: string;
  cpr_day_15m_volume_ratio?: number | null;
  cpr_day_15m_volume_surge?: boolean;
  cpr_day_ref?: string;
  next_day_date?: string;
  next_day_outcome?: string;
  next_day_bias?: string;
  next_day_summary?: string;
  next_day_prediction?: string;
  next_day_open?: number | null;
  next_day_ref?: string;
  next_day_target?: number | null;
  next_day_atr?: number | null;
  next_day_atr_pct?: number | null;
  next_day_trigger_up?: number | null;
  next_day_trigger_down?: number | null;
  next_day_pivot?: number | null;
  prev_day_high?: number | null;
  prev_day_low?: number | null;
  exp_move_up?:   number;
  exp_move_down?: number;
  exp_move_pct?:  number;
  exp_move_open_up?:  number;
  exp_move_open_dn?:  number;
  exp_move_open_pct?: number;
  day_open?: number;
  lre_score?:     number;
  lre_label?:     string;
  lre_direction?: string;
  lre_reason?:    string;
  lre_entry?:     number;
  lre_stop?:      number;
  lre_risk_pct?:  number;
  lre_status?:    string;
  lre_takeaway?:  string;
  vol_surge?:    boolean;
  breakout_score?: number;
  dist_from_high?: number;
  entry?:        number;
  stop_loss?:    number;
  target1?:      number;
  risk_pct?:     number;
  rr_t1?:        number;
  atr?:          number;
  short_pct?:    number | null;
  opt_strategy?:  string | null;
  opt_summary?:   string | null;
  opt_debit?:     number | null;
  opt_profit?:    number | null;
  opt_source?:    string | null;
  opt_quote_ts?:  string | null;
  opt_legs?:      OptLeg[] | null;
  opt_width?:     number | null;
  opt_exp_short?: string | null;
  opt_exp_long?:  string | null;
  opt_alt?:       string | null;
  opt_liquid?:    { strike: number; type: string; expiry: string; volume: number; oi: number; iv: number; otm_pct: number; vol_oi_ratio: number; unusual: boolean }[] | null;
  error?:         string | null;
  done?:         boolean;
  total?:        number;
}

const verdictColor: Record<string, string> = {
  "BULLISH":      "text-green",
  "LEAN BULLISH": "text-green/70",
  "BEARISH":      "text-red",
  "LEAN BEARISH": "text-red/70",
  "NEUTRAL":      "text-muted",
};

const biasColor: Record<string, string> = {
  BULLISH: "text-green", BEARISH: "text-red", NEUTRAL: "text-muted",
};

const gradeColor: Record<string, string> = {
  S: "bg-green/20 text-green border-green/30",
  A: "bg-green/10 text-green border-green/20",
  B: "bg-accent/10 text-accent border-accent/20",
  "B-": "bg-accent/5 text-accent border-accent/10",
  C: "bg-yellow/10 text-yellow border-yellow/20",
  D: "bg-red/5 text-muted border-border",
};

function Badge({ text, color }: { text: string; color: string }) {
  return <span className={`text-[10px] font-mono font-semibold px-1.5 py-0.5 rounded border ${color}`}>{text}</span>;
}

function SpringMarker({ title }: { title?: string }) {
  return (
    <span
      className="inline-flex h-4 w-4 items-center justify-center rounded-full border border-green/40 bg-green/10 text-[10px] leading-none text-green cursor-help"
      title={title || "Spring action"}
      aria-label={title || "Spring action"}
    >
      {"\u{1F331}"}
    </span>
  );
}

function fundamentalStyle(signal: string) {
  const s = signal.toLowerCase();
  if (
    s.includes("declining") ||
    s.includes("unprofitable") ||
    s.includes("high debt") ||
    s.includes("negative cash flow")
  ) {
    return {
      color: "#ff4d4f",
      borderColor: "rgba(255, 77, 79, 0.35)",
      backgroundColor: "rgba(255, 77, 79, 0.10)",
    };
  }
  if (
    s.includes("strong earnings") ||
    s.includes("high margins") ||
    s.includes("good dividend") ||
    s.includes("low debt") ||
    s.includes("positive cash flow") ||
    s.includes("near 52w high")
  ) {
    return {
      color: "#00e5a0",
      borderColor: "rgba(0, 229, 160, 0.35)",
      backgroundColor: "rgba(0, 229, 160, 0.10)",
    };
  }
  return {
    color: "#f5c842",
    borderColor: "rgba(245, 200, 66, 0.35)",
    backgroundColor: "rgba(245, 200, 66, 0.10)",
  };
}

function fundamentalDotStyle(signal: string) {
  const style = fundamentalStyle(signal);
  return {
    backgroundColor: style.color,
    borderColor: style.borderColor,
    boxShadow: `0 0 8px ${style.backgroundColor}`,
  };
}

function valuationClass(label?: string): string {
  if (!label) return "border-border text-muted";
  if (label === "Undervalued" || label === "Attractive") return "border-green/30 bg-green/10 text-green";
  if (label === "Expensive" || label === "Overvalued") return "border-red/30 bg-red/10 text-red";
  return "border-yellow/30 bg-yellow/10 text-yellow";
}

function valuationSortValue(r: ScanResult): number {
  if (typeof r.valuation_score === "number" && Number.isFinite(r.valuation_score)) {
    return r.valuation_score;
  }
  const label = (r.valuation_label ?? "").toLowerCase();
  if (label.includes("undervalued") || label.includes("attractive")) return 4;
  if (label.includes("fair")) return 1;
  if (label.includes("expensive")) return -2;
  if (label.includes("overvalued")) return -4;
  return -999;
}

function valuationFairValue(r: ScanResult): number | null {
  if (typeof r.valuation_fair_value === "number" && Number.isFinite(r.valuation_fair_value)) {
    return r.valuation_fair_value;
  }
  if (typeof r.price === "number" && Number.isFinite(r.price) && typeof r.valuation_score === "number" && Number.isFinite(r.valuation_score)) {
    const impliedPct = Math.max(-0.30, Math.min(0.30, r.valuation_score * 0.06));
    return Number((r.price * (1 + impliedPct)).toFixed(2));
  }
  return null;
}

function valuationUpsidePct(r: ScanResult): number | null {
  if (typeof r.valuation_upside_pct === "number" && Number.isFinite(r.valuation_upside_pct)) {
    return r.valuation_upside_pct;
  }
  const fv = valuationFairValue(r);
  if (fv != null && typeof r.price === "number" && Number.isFinite(r.price) && r.price > 0) {
    return Number((((fv - r.price) / r.price) * 100).toFixed(1));
  }
  return null;
}

function fmtMoney(value?: number | null): string {
  return typeof value === "number" && Number.isFinite(value) ? `$${value}` : "—";
}

function fmtSignedPct(value?: number | null): string {
  return typeof value === "number" && Number.isFinite(value) ? `${value >= 0 ? "+" : ""}${value.toFixed(1)}%` : "—";
}

function compactDayResult(value?: string): string {
  if (!value) return "—";
  return value
    .replace("Above CPR; pullback risk", "Above CPR")
    .replace("Below CPR; bounce risk", "Below CPR")
    .replace("Bullish above TC", "Bullish")
    .replace("Bearish below BC", "Bearish")
    .replace("Inside CPR; wait", "Wait")
    .replace("Trend up", "Trend Up")
    .replace("Trend down", "Trend Down");
}

function nextDayColor(bias?: string): string {
  if (!bias) return "text-muted";
  if (bias.includes("Above") || bias.includes("Bullish")) return "text-green";
  if (bias.includes("Below") || bias.includes("Bearish")) return "text-red";
  return "text-yellow";
}

function dayVolumeColor(value?: string): string {
  if (!value) return "text-muted";
  if (value.startsWith("Confirmed") || value.startsWith("Supportive") || value.startsWith("15m Surge") || value.startsWith("15m Active")) return "text-green";
  if (value.startsWith("Caution") || value.startsWith("15m Light")) return "text-red";
  return "text-yellow";
}

function hasPending15mVolume(r: ScanResult): boolean {
  const text = (r.cpr_day_15m_volume_text ?? "").toLowerCase();
  return !!r.ticker && !!r.cpr_day_result && (!text || text.startsWith("15m pending"));
}

function inOpeningVolumeRefreshWindow(): boolean {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: "America/New_York",
      weekday: "short",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).formatToParts(new Date());
    const get = (type: string) => parts.find(p => p.type === type)?.value ?? "";
    const weekday = get("weekday");
    if (weekday === "Sat" || weekday === "Sun") return false;
    const hour = Number(get("hour")) % 24;
    const minute = Number(get("minute"));
    if (!Number.isFinite(hour) || !Number.isFinite(minute)) return false;
    const mins = hour * 60 + minute;
    return mins >= 9 * 60 + 30 && mins <= 10 * 60 + 30;
  } catch {
    return false;
  }
}

function lreRangeText(r: ScanResult): string {
  if (r.lre_entry == null || r.lre_stop == null) return "—";
  const lo = Math.min(r.lre_entry, r.lre_stop);
  const hi = Math.max(r.lre_entry, r.lre_stop);
  return `${fmtMoney(lo)}-${fmtMoney(hi)}`;
}

function rewardPct(entry?: number | null, target?: number | null): string {
  if (!entry || !target || entry <= 0) return "—";
  return `${(Math.abs(target - entry) / entry * 100).toFixed(2)}%`;
}

function rewardPctValue(entry?: number | null, target?: number | null): number {
  if (!entry || !target || entry <= 0) return 0;
  return Math.abs(target - entry) / entry * 100;
}

function longTermFromEntryPctValue(r: ScanResult): number {
  if (!r.lre_entry || !r.price || r.lre_entry <= 0) return 0;
  return Math.abs((r.price - r.lre_entry) / r.lre_entry * 100);
}

function sectorTone(chg1d: number) {
  if (chg1d >= 0.25) return { label: "Green", dot: "bg-green", text: "text-green", border: "border-green/40", bg: "bg-green/10" };
  if (chg1d <= -0.25) return { label: "Red", dot: "bg-red", text: "text-red", border: "border-red/40", bg: "bg-red/10" };
  return { label: "Yellow", dot: "bg-yellow", text: "text-yellow", border: "border-yellow/40", bg: "bg-yellow/10" };
}

function sectorMacroKey(sector?: string): string | null {
  const s = (sector ?? "").toLowerCase();
  if (!s || s === "unknown") return null;
  if (s.includes("material") || s.includes("basic")) return "Materials";
  if (s.includes("communication") || s.includes("comm") || s.includes("telecom")) return "Comm";
  if (s.includes("energy") || s.includes("oil")) return "Energy";
  if (s.includes("financial") || s.includes("bank")) return "Financials";
  if (s.includes("industrial")) return "Industrials";
  if (s.includes("technology") || s.includes("tech") || s.includes("semiconductor") || s.includes("software")) return "Tech";
  if (s.includes("defensive") || s.includes("staple")) return "Staples";
  if (s.includes("real estate") || s.includes("reits") || s.includes("reit")) return "Real Estate";
  if (s.includes("utilit")) return "Utilities";
  if (s.includes("health") || s.includes("medical") || s.includes("biotech")) return "Health";
  if (s.includes("cyclical") || s.includes("discretionary") || s.includes("consumer")) return "Discretionary";
  return null;
}

function prevTradingDay(dateStr: string): { date: string; note: string | null } {
  const [y, m, d] = dateStr.split("-").map(Number);
  const jsDay = new Date(Date.UTC(y, m - 1, d)).getUTCDay(); // 0=Sun, 6=Sat
  if (jsDay === 6) {
    const fri = new Date(Date.UTC(y, m - 1, d - 1));
    return { date: fri.toISOString().split("T")[0], note: `Sat ${dateStr} → using Fri close` };
  }
  if (jsDay === 0) {
    const fri = new Date(Date.UTC(y, m - 1, d - 2));
    return { date: fri.toISOString().split("T")[0], note: `Sun ${dateStr} → using Fri close` };
  }
  return { date: dateStr, note: null };
}

export default function ScannerPage() {
  const [watchlist,    setWatchlist]    = useState("default");
  const [watchlistsOpen, setWatchlistsOpen] = useState(false);
  const [scannerCollapsed, setScannerCollapsed] = useState(false);
  const [houseRulesOpen, setHouseRulesOpen] = useState(true);
  const [customInput,  setCustomInput]  = useState("");
  const [tickerFilter, setTickerFilter] = useState("");
  const [scanning,     setScanning]     = useState(false);
  const [results,      setResults]      = useState<ScanResult[]>([]);
  const [progress,     setProgress]     = useState({ done: 0, total: 0 });
  const [filter,       setFilter]       = useState<Filter>("all");
  const [sortBy,       setSortBy]       = useState<SortBy>("score");
  const [optModal,     setOptModal]     = useState<{ r: ScanResult } | null>(null);
  const [otmModal,     setOtmModal]     = useState<{ r: ScanResult } | null>(null);
  const [copied,       setCopied]       = useState(false);
  const [mode,         setMode]         = useState<"live" | "backtest">("live");
  const [backtestDate, setBacktestDate] = useState("");
  const [activeBacktestDate, setActiveBacktestDate] = useState<string | null>(null);
  const [sectorMacro,  setSectorMacro]  = useState<Record<string, MacroItem>>({});
  const [auto15mStatus, setAuto15mStatus] = useState("");
  const [telegramStatus, setTelegramStatus] = useState("");
  const [telegramSending, setTelegramSending] = useState(false);
  const esRef = useRef<EventSource | null>(null);
  const refresh15mRef = useRef<EventSource | null>(null);
  const refresh15mBusyRef = useRef(false);
  const pending15mKey = results
    .filter(hasPending15mVolume)
    .map(r => r.ticker.toUpperCase())
    .sort()
    .join(",");

  useEffect(() => {
    if (!optModal) return;
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") setOptModal(null); }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [optModal]);

  useEffect(() => {
    return () => refresh15mRef.current?.close();
  }, []);

  useEffect(() => {
    let alive = true;
    fetch(`${API_BASE}/api/macro/snapshot`)
      .then(res => res.ok ? res.json() : null)
      .then(json => {
        if (!alive || !json?.items) return;
        const sectors: Record<string, MacroItem> = {};
        (json.items as MacroItem[])
          .filter(item => item.category === "sector")
          .forEach(item => { sectors[item.label] = item; });
        setSectorMacro(sectors);
      })
      .catch(() => {});
    return () => { alive = false; };
  }, []);

  useEffect(() => {
    if (mode !== "live" || scanning || progress.total === 0 || progress.done < progress.total || !pending15mKey || !inOpeningVolumeRefreshWindow()) {
      return;
    }

    const refreshPending = () => {
      if (refresh15mBusyRef.current || !inOpeningVolumeRefreshWindow()) return;
      const tickers = Array.from(new Set(
        results
          .filter(hasPending15mVolume)
          .map(r => r.ticker.toUpperCase())
      )).slice(0, 80);
      if (!tickers.length) return;

      refresh15mBusyRef.current = true;
      setAuto15mStatus(`Auto-updating 15m volume (${tickers.length})`);
      refresh15mRef.current?.close();

      const es = new EventSource(`${API_BASE}/api/scanner/stream?tickers=${encodeURIComponent(tickers.join(","))}`);
      refresh15mRef.current = es;

      es.onmessage = (e) => {
        const data: ScanResult = JSON.parse(e.data);
        if (data.done) {
          refresh15mBusyRef.current = false;
          setAuto15mStatus("15m volume refreshed");
          window.setTimeout(() => setAuto15mStatus(""), 2500);
          es.close();
          return;
        }
        if (!data.ticker) return;
        if (data.error) return;
        setResults(prev => {
          const idx = prev.findIndex(r => r.ticker.toUpperCase() === data.ticker!.toUpperCase());
          if (idx < 0) return prev;
          const next = [...prev];
          next[idx] = { ...prev[idx], ...data };
          return next;
        });
      };

      es.onerror = () => {
        refresh15mBusyRef.current = false;
        setAuto15mStatus("15m auto-update waiting");
        es.close();
      };
    };

    const first = window.setTimeout(refreshPending, 60_000);
    const every = window.setInterval(refreshPending, 90_000);
    return () => {
      window.clearTimeout(first);
      window.clearInterval(every);
    };
  }, [mode, scanning, progress.done, progress.total, pending15mKey, results]);

  function copyText(text: string) {
    navigator.clipboard.writeText(text).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  // Watchlists with daily-saved snapshots — load instantly instead of running a live scan.
  const SNAPSHOT_WATCHLISTS = ["nyse_swing", "nasdaq_swing"];
  const [snapshotStatus, setSnapshotStatus] = useState<string>("");

  async function fetchSnapshot(key: string) {
    const res = await fetch(`${API_BASE}/api/scanner/snapshot?watchlist=${key}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async function loadSnapshot(key: string) {
    setResults([]);
    setScanning(true);
    setProgress({ done: 0, total: 0 });
    setSnapshotStatus("Loading saved snapshot…");
    try {
      let json = await fetchSnapshot(key);

      // Auto-trigger if no snapshot exists yet (first-time use, before 3 PM cron has fired).
      if (!json.available || !json.results || json.results.length === 0) {
        setSnapshotStatus("No snapshot yet — building it now (~1–3 minutes)…");
        const trig = await fetch(`${API_BASE}/api/scanner/snapshot/run?watchlist=${key}`, { method: "POST" });
        if (!trig.ok && trig.status !== 202) throw new Error(`Trigger failed: HTTP ${trig.status}`);

        // Poll every 10s up to 5 minutes
        const maxAttempts = 30;
        for (let i = 0; i < maxAttempts; i++) {
          await new Promise(r => setTimeout(r, 10_000));
          setSnapshotStatus(`Building snapshot… (${i * 10}s)`);
          json = await fetchSnapshot(key);
          if (json.available && json.results && json.results.length > 0) break;
          if (i === maxAttempts - 1) {
            throw new Error("Snapshot build timed out after 5 minutes");
          }
        }
      }

      setResults(json.results);
      setProgress({ done: json.count, total: json.count });
      setActiveBacktestDate(json.date);
      setSnapshotStatus("");
      setScannerCollapsed(true);
    } catch (e: any) {
      setSnapshotStatus(`Failed: ${e?.message ?? e}`);
    } finally {
      setScanning(false);
    }
  }

  function startScan(scanWatchlist: string = watchlist) {
    if (esRef.current) esRef.current.close();
    if (refresh15mRef.current) refresh15mRef.current.close();
    refresh15mBusyRef.current = false;
    setAuto15mStatus("");
    setSnapshotStatus("");
    setScannerCollapsed(false);
    if (scanWatchlist === "custom" && watchlist !== "custom") {
      setWatchlist("custom");
    }

    // For NYSE/NASDAQ swing in live mode, prefer the saved snapshot (faster).
    if (mode === "live" && SNAPSHOT_WATCHLISTS.includes(scanWatchlist)) {
      loadSnapshot(scanWatchlist);
      return;
    }

    setResults([]);

    let url: string;
    let total: number;

    if (scanWatchlist === "custom") {
      const tickers = customInput.split(",").map(t => t.trim().toUpperCase()).filter(Boolean);
      if (!tickers.length) return;
      total = tickers.length;
      url = `${API_BASE}/api/scanner/stream?tickers=${encodeURIComponent(tickers.join(","))}`;
    } else {
      total = WATCHLISTS.find(w => w.key === scanWatchlist)?.count ?? 50;
      url = `${API_BASE}/api/scanner/stream?watchlist=${scanWatchlist}`;
    }

    if (mode === "backtest" && backtestDate) {
      url += `&as_of=${backtestDate}`;
      setActiveBacktestDate(backtestDate);
    } else {
      setActiveBacktestDate(null);
    }

    setProgress({ done: 0, total });
    setScanning(true);

    const es = new EventSource(url);
    esRef.current = es;

    es.onmessage = (e) => {
      const data: ScanResult = JSON.parse(e.data);
      if (data.done) {
        setScanning(false);
        setProgress(p => ({ ...p, done: data.total ?? p.done }));
        setScannerCollapsed(true);
        es.close();
        return;
      }
      setResults(prev => [...prev, data]);
      setProgress(p => ({ ...p, done: p.done + 1 }));
    };

    es.onerror = () => {
      setScanning(false);
      es.close();
    };
  }

  function stopScan() {
    esRef.current?.close();
    setScanning(false);
  }

  async function sendTelegramAlert() {
    setTelegramSending(true);
    setTelegramStatus("Scanning Default 50 for lightning...");
    try {
      const res = await fetch(`${API_BASE}/api/telegram/lightning-scan`, { method: "POST" });
      const json = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(json?.error || json?.detail || `HTTP ${res.status}`);
      setTelegramStatus(json?.message || "Telegram alert sent");
      window.setTimeout(() => setTelegramStatus(""), 6000);
    } catch (e: any) {
      setTelegramStatus(`Telegram failed: ${e?.message ?? e}`);
    } finally {
      setTelegramSending(false);
    }
  }

  const gradeRank: Record<string, number> = { S: 0, A: 1, B: 2, "B-": 3, C: 4, D: 5 };
  const tickerFilterTokens = tickerFilter
    .split(/[\s,]+/)
    .map(t => t.trim().toUpperCase())
    .filter(Boolean);

  const filtered = results
    .filter(r => !r.error && r.verdict)
    .filter(r => tickerFilterTokens.length === 0 || tickerFilterTokens.includes(r.ticker.toUpperCase()))
    .filter(r => {
      if (filter === "rank1")     return r.mtf_rank === 1;
      if (filter === "high_short") return (r.short_pct ?? 0) >= 10;
      // Matches original: score≥4 + HIGH conf + grade A/S + rank1 + ACCUMULATING vol
      if (filter === "exceptional")
        return ["S", "A"].includes(r.entry_grade ?? "")
          && r.mtf_rank === 1
          && r.vol_trend === "ACCUMULATING";
      // Actionable = MTF rank 1 + LRE entry zone live (ACTIVE or DISCOUNT)
      if (filter === "actionable")
        return r.mtf_rank === 1
          && (r.lre_status === "ACTIVE" || r.lre_status === "DISCOUNT");
      return true;
    })
    .sort((a, b) => {
      if (sortBy === "score")  return Math.abs(b.score ?? 0) - Math.abs(a.score ?? 0);
      if (sortBy === "grade")  return (gradeRank[a.entry_grade ?? "D"] ?? 5) - (gradeRank[b.entry_grade ?? "D"] ?? 5);
      if (sortBy === "rr")     return (b.rr_t1 ?? 0) - (a.rr_t1 ?? 0);
      if (sortBy === "swingReward") return rewardPctValue(b.entry, b.target1) - rewardPctValue(a.entry, a.target1);
      if (sortBy === "dayReward")   return rewardPctValue(b.cpr_day_entry, b.cpr_day_t1) - rewardPctValue(a.cpr_day_entry, a.cpr_day_t1);
      if (sortBy === "ltEntryPct")  return longTermFromEntryPctValue(b) - longTermFromEntryPctValue(a);
      if (sortBy === "valuation")   return valuationSortValue(b) - valuationSortValue(a);
      return 0;
    });

  const errors  = results.filter(r => r.error);
  const pct     = progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0;
  const selectedWatchlist = WATCHLISTS.find(w => w.key === watchlist);
  const customTickerCount = customInput.split(",").filter(t => t.trim()).length;

  return (
    <div className="space-y-4">

      {/* ── Header ── */}
      {/* ── Controls ── */}
      <div className="card space-y-3">
        <div className="flex items-center gap-3 overflow-x-auto rounded-lg border border-border bg-card/40 px-3 py-2">
          <span className="shrink-0 text-lg font-bold text-white">Scanner</span>
          <button
            type="button"
            onClick={() => {
              setScannerCollapsed(false);
              setWatchlistsOpen(v => !v);
            }}
            className="inline-flex shrink-0 items-center gap-2 rounded-lg border border-border bg-surface/50 px-2 py-1.5 text-left hover:border-white/20"
          >
            <span className="inline-flex shrink-0 items-center gap-2">
              <span className="rounded border border-border bg-surface px-2 py-1 text-[11px] font-bold uppercase tracking-wide text-muted">
                Watch List
              </span>
              <span className="rounded border border-border bg-card px-2 py-1 text-xs font-semibold text-white">
                {selectedWatchlist?.label ?? watchlist}
              </span>
              <span className="text-xs text-muted">
                {watchlist === "custom"
                  ? `${customTickerCount} ticker${customTickerCount !== 1 ? "s" : ""}`
                  : `${selectedWatchlist?.count ?? 0} tickers`}
              </span>
            </span>
            <span className={`whitespace-nowrap rounded border px-2 py-1 text-xs font-bold transition-colors ${
              watchlistsOpen
                ? "border-accent/50 bg-accent/15 text-accent"
                : "border-yellow/40 bg-yellow/10 text-yellow"
            }`}>
              {watchlistsOpen ? "Hide" : "Options"}
            </span>
          </button>
          <span className="shrink-0 rounded border border-border bg-surface px-2 py-1 text-[11px] font-bold uppercase tracking-wide text-muted">
            Mode
          </span>
          <div className="flex shrink-0 rounded-lg border border-border bg-surface overflow-hidden">
            <button
              onClick={() => setMode("live")}
              className={`px-4 py-1.5 text-xs font-semibold transition-colors ${
                mode === "live" ? "bg-accent text-black" : "text-muted hover:text-white bg-transparent"
              }`}
            >
              ▶ Live
            </button>
            <button
              onClick={() => setMode("backtest")}
              className={`px-4 py-1.5 text-xs font-semibold transition-colors border-l border-border ${
                mode === "backtest" ? "bg-accent text-black" : "text-muted hover:text-white bg-transparent"
              }`}
            >
              ⏪ Backtest
            </button>
          </div>
          <button
            onClick={scanning ? stopScan : () => startScan()}
            disabled={!scanning && mode === "backtest" && !backtestDate}
            className={`shrink-0 px-6 py-1.5 rounded-lg font-bold text-sm uppercase tracking-wide transition-all ${
              scanning
                ? "bg-red/20 text-red border border-red/30 hover:bg-red/30"
                : mode === "backtest" && !backtestDate
                  ? "bg-surface text-muted border border-border cursor-not-allowed"
                  : "bg-accent text-black border border-accent hover:bg-accent/85"
            }`}
          >
            {scanning ? "⏹ STOP" : mode === "backtest" ? "⏪ BACKTEST" : "▶ SCAN"}
          </button>
          <button
            type="button"
            onClick={sendTelegramAlert}
            disabled={telegramSending}
            className={`shrink-0 rounded-lg border px-3 py-1.5 text-xs font-bold uppercase transition-colors ${
              telegramSending
                ? "border-yellow/30 bg-yellow/10 text-yellow cursor-wait"
                : "border-border bg-surface text-muted hover:border-yellow/40 hover:text-yellow"
            }`}
            title="Force scan Default 50 and send lightning-volume options plays to Telegram"
          >
            {telegramSending ? "TG..." : "TG Scan"}
          </button>
          {(scanning || results.length > 0 || snapshotStatus || telegramStatus) && (
            <div className="shrink-0 min-w-[230px] max-w-[280px]">
              <div className="mb-1 flex items-center justify-between gap-3 text-[11px] text-muted">
                <span>{progress.done} / {progress.total} scanned</span>
                <span>{pct}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-surface">
                <div className="h-full rounded-full bg-accent/80 transition-all duration-300"
                     style={{ width: `${pct}%` }} />
              </div>
              <div className="mt-1 flex flex-wrap gap-x-2 text-[10px] text-muted">
                {results.length > 0 && !scanning && (
                  <span>{filtered.length} shown · {errors.length} errors</span>
                )}
                {snapshotStatus && <span className="text-accent">{snapshotStatus}</span>}
                {auto15mStatus && <span className="text-yellow">{auto15mStatus}</span>}
                {telegramStatus && (
                  <span className={telegramStatus.startsWith("Telegram failed") ? "text-red" : "text-green"}>
                    {telegramStatus}
                  </span>
                )}
              </div>
            </div>
          )}
          {results.length > 0 && !scanning && (
            <button
              type="button"
              onClick={() => setScannerCollapsed(v => !v)}
              className="shrink-0 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-muted hover:border-white/20 hover:text-white"
            >
              {scannerCollapsed ? "Expand" : "Collapse"}
            </button>
          )}
        </div>

        {!scannerCollapsed && watchlistsOpen && (
          <div className="flex flex-wrap gap-2 rounded-lg border border-border bg-surface/30 px-3 py-2">
            {WATCHLISTS.map(w => {
              if (w.key === "custom" && watchlist === "custom") {
                return (
                  <div key={w.key} className="flex items-center gap-2 rounded-lg border border-accent/50 bg-surface px-2 py-1">
                    <span className="text-xs font-semibold text-accent">Custom</span>
                    <input
                      autoFocus
                      type="text"
                      value={customInput}
                      onChange={e => setCustomInput(e.target.value)}
                      onKeyDown={e => e.key === "Enter" && !scanning && customInput.trim() && startScan("custom")}
                      placeholder="AAPL, MSFT"
                      className="w-44 rounded border border-border bg-transparent px-2 py-1 text-xs font-mono text-white placeholder-muted focus:border-accent focus:outline-none"
                    />
                    <button
                      onClick={scanning ? stopScan : () => startScan("custom")}
                      disabled={!scanning && (!customInput.trim() || (mode === "backtest" && !backtestDate))}
                      className={`rounded px-3 py-1 text-xs font-bold transition-colors ${
                        scanning
                          ? "border border-red/30 bg-red/20 text-red hover:bg-red/30"
                          : !customInput.trim() || (mode === "backtest" && !backtestDate)
                            ? "border border-border bg-card text-muted cursor-not-allowed"
                            : "border border-accent bg-accent text-black hover:bg-accent/85"
                      }`}
                    >
                      {scanning ? "STOP" : "SCAN"}
                    </button>
                  </div>
                );
              }
              return (
                <button key={w.key} onClick={() => setWatchlist(w.key)}
                  className={`px-3 py-1.5 text-xs rounded-lg font-semibold border transition-colors ${
                    watchlist === w.key
                      ? "bg-accent text-black border-accent"
                      : "border-border text-muted hover:text-white hover:border-white/20"
                  }`}>
                  {w.label}
                </button>
              );
            })}
          </div>
        )}

        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex min-w-[320px] max-w-2xl flex-1 items-center gap-2">
              <span className="shrink-0 rounded border border-accent/30 bg-accent/10 px-2 py-1 text-[10px] font-bold uppercase tracking-wide text-accent">
                Custom
              </span>
              <input
                type="text"
                value={customInput}
                onChange={e => setCustomInput(e.target.value)}
                onFocus={() => setWatchlist("custom")}
                onKeyDown={e => e.key === "Enter" && !scanning && customInput.trim() && startScan("custom")}
                placeholder="One or many tickers: AAPL, MSFT"
                className="min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs font-mono text-white placeholder-muted focus:border-accent focus:outline-none"
              />
              <button
                onClick={scanning ? stopScan : () => startScan("custom")}
                disabled={!scanning && (!customInput.trim() || (mode === "backtest" && !backtestDate))}
                className={`shrink-0 rounded-lg px-4 py-1.5 text-xs font-bold uppercase transition-colors ${
                  scanning
                    ? "bg-red/20 text-red border border-red/30 hover:bg-red/30"
                    : !customInput.trim() || (mode === "backtest" && !backtestDate)
                      ? "bg-surface text-muted border border-border cursor-not-allowed"
                      : "bg-accent text-black border border-accent hover:bg-accent/85"
                }`}
              >
                {scanning ? "Stop" : "Scan"}
              </button>
              {customInput && (
                <span className="shrink-0 text-xs text-muted">
                  {customInput.split(",").filter(t => t.trim()).length} ticker{customInput.split(",").filter(t => t.trim()).length !== 1 ? "s" : ""}
                </span>
              )}
            </div>

            {mode === "backtest" && (
              <div className="flex flex-wrap gap-2 items-center">
                <span className="text-xs text-muted">As of date:</span>
                <input
                  type="date"
                  value={backtestDate}
                  max={new Date(Date.now() - 86400000).toISOString().split("T")[0]}
                  onChange={e => setBacktestDate(e.target.value)}
                  className="bg-surface border border-border rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-accent font-mono"
                />
                {!backtestDate && (
                  <span className="text-xs text-yellow">Pick a date to backtest</span>
                )}
                {backtestDate && prevTradingDay(backtestDate).note && (
                  <span className="text-xs text-yellow">{prevTradingDay(backtestDate).note}</span>
                )}
              </div>
            )}

            {/* ── Mode toggle ── */}
            <div className="hidden">
              <span className="rounded border border-accent/40 bg-accent/15 px-2 py-1 text-[11px] font-bold uppercase tracking-wide text-accent">
                Mode
              </span>
              <div className="flex rounded-lg border border-accent/40 bg-accent/5 shadow-[0_0_14px_rgba(96,165,250,0.12)] overflow-hidden">
                <button
                  onClick={() => setMode("live")}
                  className={`px-4 py-1.5 text-xs font-semibold transition-colors ${
                    mode === "live" ? "bg-accent text-black" : "text-muted hover:text-white bg-transparent"
                  }`}
                >
                  ▶ Live
                </button>
                <button
                  onClick={() => setMode("backtest")}
                  className={`px-4 py-1.5 text-xs font-semibold transition-colors border-l border-border ${
                    mode === "backtest" ? "bg-accent text-black" : "text-muted hover:text-white bg-transparent"
                  }`}
                >
                  ⏪ Backtest
                </button>
              </div>
              <button
                onClick={scanning ? stopScan : () => startScan()}
                disabled={!scanning && mode === "backtest" && !backtestDate}
                className={`px-6 py-1.5 rounded-lg font-bold text-sm uppercase tracking-wide transition-all ${
                  scanning
                    ? "bg-red/20 text-red border border-red/30 shadow-[0_0_16px_rgba(248,113,113,0.25)] hover:bg-red/30"
                    : mode === "backtest" && !backtestDate
                      ? "bg-surface text-muted border border-border cursor-not-allowed"
                      : "bg-accent text-black ring-1 ring-accent/60 shadow-[0_0_20px_rgba(96,165,250,0.35)] hover:bg-accent/90 hover:shadow-[0_0_26px_rgba(96,165,250,0.5)]"
                }`}
              >
                {scanning ? "⏹ STOP" : mode === "backtest" ? "⏪ BACKTEST" : "▶ SCAN"}
              </button>
              {mode === "backtest" && (
                <div className="flex flex-wrap gap-2 items-center">
                  <span className="text-xs text-muted">As of date:</span>
                  <input
                    type="date"
                    value={backtestDate}
                    max={new Date(Date.now() - 86400000).toISOString().split("T")[0]}
                    onChange={e => setBacktestDate(e.target.value)}
                    className="bg-surface border border-border rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-accent font-mono"
                  />
                  {!backtestDate && (
                    <span className="text-xs text-yellow">Pick a date to backtest</span>
                  )}
                  {backtestDate && prevTradingDay(backtestDate).note && (
                    <span className="text-xs text-yellow">{prevTradingDay(backtestDate).note}</span>
                  )}
                </div>
              )}
            </div>

            {false && (scanning || results.length > 0 || snapshotStatus) && (
            <div className="flex gap-3 items-center">
              {(scanning || results.length > 0) && (
                <div className="flex-1 max-w-xs">
                  <div className="flex justify-between text-xs text-muted mb-1">
                    <span>{progress.done} / {progress.total} scanned</span>
                    <span>{pct}%</span>
                  </div>
                  <div className="h-1.5 bg-surface rounded-full overflow-hidden">
                    <div className="h-full bg-accent transition-all duration-300 rounded-full"
                         style={{ width: `${pct}%` }} />
                  </div>
                </div>
              )}

              {results.length > 0 && !scanning && (
                <span className="text-xs text-muted">
                  {filtered.length} shown · {errors.length} errors
                </span>
              )}

              {/* Snapshot loading status (NYSE/NASDAQ swing) */}
              {snapshotStatus && (
                <span className="text-xs text-accent">{snapshotStatus}</span>
              )}
            </div>
            )}
          </div>

          <div className="rounded-lg border border-border bg-surface/45 px-3 py-2 text-[10px] text-muted">
            <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
              <span className="text-xs font-semibold text-white">House Rules</span>
              <div className="flex items-center gap-2">
                <span className="rounded border border-yellow/30 bg-yellow/10 px-1.5 py-0.5 text-[9px] font-semibold text-yellow">
                  Not financial advice
                </span>
                <button
                  type="button"
                  onClick={() => setHouseRulesOpen(v => !v)}
                  className="rounded border border-border px-2 py-0.5 text-[9px] font-semibold text-muted hover:border-white/20 hover:text-white"
                >
                  {houseRulesOpen ? "Collapse" : "Expand"}
                </button>
              </div>
            </div>
            {houseRulesOpen && (
            <div className="grid grid-cols-1 gap-x-4 gap-y-1.5 sm:grid-cols-2 xl:grid-cols-4">
              {[
                "Respect stop loss; no averaging after invalidation.",
                "Plan entry, target, size, and exit before taking the trade.",
                "Always check the \"lightning symbol\" (⚡) for unusual volume tickers.",
                "For day trades, confirm market sentiment before entry.",
                "Day trades need liquidity, confirmation, and smaller risk.",
                "Options can expire worthless; favor defined risk and liquid chains.",
                "Do not chase gaps; wait for trigger or clean retest.",
                "Scanner output is a decision aid, not a trade command.",
              ].map(rule => (
                <div key={rule} className="flex gap-2 leading-snug">
                  <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent/80" />
                  <span>{rule}</span>
                </div>
              ))}
            </div>
            )}
          </div>
        </div>
      </div>

      {/* ── Backtest banner ── */}
      {activeBacktestDate && results.length > 0 && (() => {
        const { date: effectiveDate, note } = prevTradingDay(activeBacktestDate);
        return (
          <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent/10 border border-accent/20 text-sm">
            <span className="text-accent font-semibold">⏪ Backtest</span>
            <span className="text-muted">Showing scanner results as of</span>
            <span className="font-mono text-white">{effectiveDate}</span>
            {note && <span className="text-xs text-yellow ml-1">({note})</span>}
          </div>
        );
      })()}

      {/* ── Filter + Sort ── */}
      {results.length > 0 && (
        <div className="flex flex-wrap gap-4 items-center">
          <div className="flex gap-1 bg-card border border-border rounded-lg p-1">
            {(["all", "actionable", "rank1", "exceptional", "high_short"] as Filter[]).map(f => (
              <button key={f} onClick={() => setFilter(f)}
                className={`px-3 py-1 text-xs rounded-md font-semibold transition-colors ${
                  filter === f ? "bg-accent text-black" : "text-muted hover:text-white"
                }`}>
                {f === "all"         ? `All (${results.filter(r => !r.error).length})`
                : f === "actionable" ? `🎯 Actionable (${results.filter(r => r.mtf_rank === 1 && (r.lre_status === "ACTIVE" || r.lre_status === "DISCOUNT")).length})`
                : f === "rank1"      ? `Rank 1 (${results.filter(r => r.mtf_rank === 1).length})`
                : f === "high_short" ? `🔥 High Short (${results.filter(r => (r.short_pct ?? 0) >= 10).length})`
                : `Exceptional (${results.filter(r => ["S","A"].includes(r.entry_grade ?? "") && r.mtf_rank === 1 && r.vol_trend === "ACCUMULATING").length})`}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-1 text-xs text-muted">
            <span>Sort:</span>
            {SORT_OPTIONS.map(s => (
              <button key={s.key} onClick={() => setSortBy(s.key)} title={s.title}
                className={`px-2 py-1 rounded transition-colors ${
                  sortBy === s.key ? "text-white font-semibold" : "hover:text-white"
                }`}>
                {s.label}
              </button>
            ))}
          </div>
          <div className="flex min-w-[230px] max-w-sm flex-1 items-center gap-2">
            <span className="shrink-0 text-xs text-muted">Filter:</span>
            <input
              type="text"
              value={tickerFilter}
              onChange={e => setTickerFilter(e.target.value)}
              placeholder="AAPL, MSFT"
              className="min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs font-mono text-white placeholder-muted focus:border-accent focus:outline-none"
            />
          </div>

          <button
            onClick={() => downloadCsv(
              `scanner_${activeBacktestDate ?? "live"}.csv`,
              [
                "Ticker","Sector","Price","Verdict","Long Term Grade","Long Term Status",
                "Verdict Flip Date","Verdict Flip From","Days Since Flip",
                "Long Term Entry Range","Long Term % From Entry","Long Term Risk%","Long Term Spring","Valuation","Valuation Fair Value","Valuation Upside%","Valuation Source","Valuation Reason",
                "Swing Entry","Swing Stop","Swing T1","Swing Reward%","Swing Risk%","Swing R/R","Swing Spring",
                "Day Trading Result","Day Trading Entry","Day Trading Stop","Day Trading T1","Day Trading Reward%","Day Trading Spring","Day Trading Trigger","Day Trading Invalidation","Day Trading Target Plan","Day Trading Volume Confirm","Day Trading 15m Volume Confirm","Day Trading Ref",
                "Next Day Date","Next Day Outcome","Next Day Bias","Next Day Summary","Next Day ATR","Next Day ATR%","Next Day Up Trigger","Next Day Down Trigger","Next Day Pivot","Next Day Target",
                "Short%","Options Strategy","Options Summary","Fundamental","CPR Text",
              ],
              filtered.map(r => {
                const lreFromEntry = r.lre_entry && r.price
                  ? `${(((r.price - r.lre_entry) / r.lre_entry) * 100).toFixed(1)}%`
                  : null;
                return [
                  r.ticker, r.sector, r.price, r.verdict, r.lre_label, r.lre_status,
                  r.verdict_flip_date, r.verdict_flip_from, r.verdict_flip_days,
                  lreRangeText(r), lreFromEntry, r.lre_risk_pct, r.long_term_spring_text, r.valuation_label, valuationFairValue(r), valuationUpsidePct(r), r.valuation_source, r.valuation_reason,
                  r.entry, r.stop_loss, r.target1, rewardPct(r.entry, r.target1), r.risk_pct, r.rr_t1, r.swing_spring_text,
                  r.cpr_day_result, r.cpr_day_entry, r.cpr_day_stop, r.cpr_day_t1, rewardPct(r.cpr_day_entry, r.cpr_day_t1),
                  r.day_spring_text, r.cpr_day_trigger_text, r.cpr_day_invalidation_text, r.cpr_day_target_text, r.cpr_day_volume_text, r.cpr_day_15m_volume_text, r.cpr_day_ref,
                  r.next_day_date, r.next_day_outcome, r.next_day_bias, r.next_day_summary ?? r.next_day_prediction,
                  r.next_day_atr, r.next_day_atr_pct, r.next_day_trigger_up, r.next_day_trigger_down,
                  r.next_day_pivot, r.next_day_target,
                  r.short_pct != null ? Math.round(r.short_pct) : null,
                  r.opt_strategy, [r.opt_summary, r.opt_alt].filter(Boolean).join(" | "),
                  r.signals, r.cpr_interpretation,
                ];
              })
            )}
            className="ml-auto px-3 py-1.5 text-xs rounded-lg border border-border text-muted hover:text-white hover:border-white/20 transition-colors"
          >
            ⬇ CSV
          </button>
        </div>
      )}

      {/* ── Results Table ── */}
      {filtered.length > 0 && (
        <div className="card p-0 overflow-hidden">
          <div className="max-h-[72vh] overflow-auto">
            <table className="min-w-full table-auto text-sm" style={{ borderCollapse: "collapse", width: "max-content" }}>
              <thead className="sticky top-0 z-30 bg-card">
                <tr className="border-b border-border bg-card text-muted text-xs shadow-[0_1px_0_rgba(148,163,184,0.2)]">
                  {/* sticky ticker column */}
                  <th className="text-left pl-4 pr-3 py-3 whitespace-nowrap sticky left-0 bg-card z-40">Ticker</th>
                  <th className="w-[88px] min-w-[88px] max-w-[88px] text-center px-2 py-3">Sector</th>
                  <th className="text-right px-3 py-3 whitespace-nowrap">Price</th>
                  <th className="text-center px-3 py-3 whitespace-nowrap">Verdict</th>
                  <th className="text-center px-3 py-3 whitespace-nowrap" title="Long Term scans weekly bars for spring action. A green sprout appears in rows when detected.">
                    Long Term <span className="text-green/60">{"\u{1F331}"}</span>
                  </th>
                  <th className="text-left px-2 py-3 whitespace-nowrap border-l border-border/60 text-accent" title="Swing scans daily bars for spring action. A green sprout appears in rows when detected.">
                    SWING <span className="text-green/60">{"\u{1F331}"}</span>
                  </th>
                  <th className="text-left px-2 py-3 whitespace-nowrap border-l border-border/60 text-yellow" title="Day Trading scans 4H bars for spring action. A green sprout appears in rows when detected.">
                    Day Trading <span className="text-green/60">{"\u{1F331}"}</span>
                  </th>
                  <th className="text-left px-2 py-3 whitespace-nowrap border-l border-border/60 text-muted" title="Prediction only. Use with caution and confirm with price action.">
                    <span className="block leading-tight">
                      <span className="block">Next Day</span>
                      <span className="block text-[9px] font-normal text-yellow">(Prediction/use with Caution)</span>
                    </span>
                  </th>
                  <th className="w-[42px] text-right px-1.5 py-3 whitespace-nowrap">Short%</th>
                  <th className="text-left px-3 py-3 whitespace-nowrap">Options</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((r) => {
                  const optEmoji = r.opt_strategy?.includes("Bull") ? "📈"
                                 : r.opt_strategy?.includes("Bear") ? "📉"
                                 : r.opt_strategy?.includes("Butterfly") ? "🦋"
                                 : r.opt_strategy?.includes("Straddle") ? "🦋"
                                 : r.opt_strategy ? "📊" : null;
                  const optShort = r.opt_strategy
                    ?.replace("Bull Call Spread", "BCS")
                     .replace("Bear Put Spread",  "BPS")
                     .replace("Iron Butterfly",   "Iron Fly")
                     .replace("Long Call",        "Long C")
                     .replace("Long Put",         "Long P");
                  const optAltText = `${r.opt_summary ?? ""}\n${r.opt_alt ?? ""}`;
                  const hasZebra = optAltText.includes("ZEBRA");
                  const hasButterflyAlt = optAltText.includes("Butterfly") && !r.opt_strategy?.includes("Butterfly");
                  const topCall = r.opt_liquid?.find(c => c.type === "CALL");
                  const topPut = r.opt_liquid?.find(c => c.type === "PUT");
                  const hasOtmData = topCall || topPut;
                  const otmInterp = hasOtmData ? interpretOtmFlow(r.opt_liquid ?? []) : "";
                  const sectorKey = sectorMacroKey(r.sector);
                  const sectorItem = sectorKey ? sectorMacro[sectorKey] : undefined;
                  const sectorToneInfo = sectorItem ? sectorTone(sectorItem.chg_1d) : null;
                  const sectorSign = sectorItem && sectorItem.chg_1d > 0 ? "+" : "";
                  return (
                    <tr key={r.ticker}
                      className="border-b border-border/40 hover:bg-surface/50 transition-colors">
                      <td className="pl-4 pr-3 py-2.5 whitespace-nowrap sticky left-0 bg-card">
                        <Link href={`/stock/${r.ticker}`}
                          className="font-bold text-white hover:text-accent transition-colors">
                          {r.ticker}
                        </Link>
                        {r.vol_surge && <span className="ml-1 text-[10px] text-yellow">⚡</span>}
                      </td>
                      <td className="w-[88px] min-w-[88px] max-w-[88px] px-2 py-2.5 text-center align-middle">
                        {r.sector && r.sector !== "Unknown" ? (
                          <span
                            className={`inline-flex w-[76px] items-center justify-center gap-1 rounded border px-1 py-0.5 text-[10px] leading-tight whitespace-normal break-words ${
                              sectorToneInfo
                                ? `${sectorToneInfo.border} ${sectorToneInfo.bg} ${sectorToneInfo.text}`
                                : "border-border/60 bg-surface/60 text-muted"
                            }`}
                            title={
                              sectorItem
                                ? `${r.sector}: sector ETF ${sectorItem.ticker} ${sectorSign}${sectorItem.chg_1d.toFixed(2)}% 1D. This is sector sentiment, not the ticker verdict.`
                                : r.sector
                            }
                          >
                            {sectorToneInfo && <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${sectorToneInfo.dot}`} />}
                            <span>{r.sector}</span>
                          </span>
                        ) : (
                          <span className="text-muted/40 text-xs">—</span>
                        )}
                          {/*

                          {hasOtmData && (
                            <div
                              className="cursor-pointer select-none"
                              onDoubleClick={() => setOtmModal({ r })}
                              title={otmInterp || "Double-click to view all OTM contracts"}
                            >
                              <span className="flex flex-col gap-0.5">
                                {[topCall, topPut].filter(Boolean).map((c, i) => {
                                  const cc = c!.type === "CALL" ? "text-green" : "text-red";
                                  return (
                                    <span key={i} className={`flex items-center gap-1 text-[10px] font-mono ${c!.unusual ? "bg-yellow/5 rounded px-0.5" : ""}`}>
                                      {c!.unusual && <span className="text-yellow text-[8px]">⚡</span>}
                                      <span className={`font-bold ${cc}`}>{c!.type[0]}</span>
                                      <span className="text-white">${c!.strike}</span>
                                      <span className="text-muted">{c!.otm_pct}%otm</span>
                                      <span className={c!.vol_oi_ratio > 0.5 ? "text-yellow" : "text-muted"}>
                                        {c!.volume >= 1000 ? `${(c!.volume / 1000).toFixed(1)}K` : c!.volume}v
                                      </span>
                                    </span>
                                  );
                                })}
                              </span>
                            </div>
                          )}
                          */}
                      </td>
                      <td className="px-3 py-2.5 text-right font-mono text-white whitespace-nowrap">
                        ${r.price?.toFixed(2)}
                      </td>
                      <td className="px-3 py-2.5 text-center whitespace-nowrap" title={r.lre_reason ?? ""}>
                        <div className="flex flex-col items-center leading-tight gap-0.5">
                          <span className={`text-xs font-semibold ${verdictColor[r.verdict ?? ""] ?? "text-muted"}`}>
                            {r.verdict}
                          </span>
                          {r.verdict_flip_text && (
                            <span
                              className="max-w-[62px] whitespace-normal text-[9px] font-mono text-yellow cursor-help"
                              title={`${r.verdict_flip_text}${r.verdict_flip_days != null ? ` (${r.verdict_flip_days} days ago)` : ""}`}
                            >
                              Flip{r.verdict_flip_days != null ? ` (${r.verdict_flip_days}d)` : ""}
                            </span>
                          )}
                          {r.lre_score && r.lre_score > 0 && (
                            <>
                              <span className={`text-xs font-bold ${
                                r.lre_score >= 5 ? "text-yellow" :
                                r.lre_score >= 4 ? "text-green"  :
                                r.lre_score >= 3 ? "text-accent" :
                                                    "text-muted"
                              }`}>
                                {"★".repeat(r.lre_score)}<span className="text-muted/30">{"☆".repeat(5 - r.lre_score)}</span>
                              </span>
                              <span className={`text-[9px] font-mono ${
                                r.lre_direction === "LONG"  ? "text-green" :
                                r.lre_direction === "SHORT" ? "text-red"   :
                                                              "text-muted"
                              }`}>
                                {r.lre_label}
                              </span>
                              {r.lre_status && (() => {
                                const styles: Record<string, string> = {
                                  ACTIVE:      "bg-green/15 text-green border-green/30",
                                  DISCOUNT:    "bg-accent/15 text-accent border-accent/30",
                                  STALE:       "bg-yellow/15 text-yellow border-yellow/30",
                                  INVALIDATED: "bg-red/15 text-red border-red/30",
                                };
                                return (
                                  <span className={`text-[8px] font-bold px-1.5 py-0.5 rounded border ${styles[r.lre_status!] ?? "text-muted border-border"}`}>
                                    {r.lre_status}
                                  </span>
                                );
                              })()}
                            </>
                          )}
                        </div>
                      </td>
                      <td className="px-3 py-2.5 text-center whitespace-nowrap" title={r.lre_reason ?? ""}>
                        {r.long_term_spring && (
                          <div className="mb-1 inline-flex items-center justify-center gap-1 text-[10px] font-mono text-green">
                            <SpringMarker title={r.long_term_spring_text} />
                            Weekly spring
                          </div>
                        )}
                        {r.lre_score && r.lre_score > 0 ? (
                          <div className="flex flex-col items-center leading-tight gap-0.5">
                            {r.lre_entry != null && r.lre_stop != null && (
                              <div className="flex flex-col items-center gap-0.5 font-mono">
                                <span className="text-[9px] text-muted">Entry Range</span>
                                <span className="text-[10px] text-white">{lreRangeText(r)}</span>
                              </div>
                            )}
                            {r.lre_entry != null && r.price != null && r.lre_entry > 0 && (() => {
                              const diffPct = ((r.price! - r.lre_entry!) / r.lre_entry!) * 100;
                              const stale = Math.abs(diffPct) > 5;
                              const dirLong = r.lre_direction === "LONG";
                              // For long: positive diff = price above entry = need pullback (stale)
                              // For short: negative diff = price below entry = need bounce (stale)
                              const needsPullback = (dirLong && diffPct > 5) || (!dirLong && diffPct < -5);
                              const sign = diffPct > 0 ? "+" : "";
                              return (
                                <span className={`text-[9px] font-mono ${
                                  needsPullback ? "text-yellow" :
                                  stale          ? "text-muted/60" :
                                                   "text-green/80"
                                }`}>
                                  {sign}{diffPct.toFixed(1)}% from entry
                                </span>
                              );
                            })()}
                            {r.lre_risk_pct != null && (
                              <span className="text-[9px] font-mono text-muted">
                                Risk {r.lre_risk_pct.toFixed(2)}%
                              </span>
                            )}
                          </div>
                        ) : (
                          <span className="text-muted/40 text-xs">—</span>
                        )}
                        {r.signals && (
                          <div className="mt-1 flex max-w-[90px] flex-wrap justify-center gap-1 leading-tight whitespace-normal" title={r.signals}>
                            {r.signals.split(" | ").map((signal, idx) => (
                              <span
                                key={`${r.ticker}-signal-${idx}`}
                                className="inline-block h-2.5 w-2.5 rounded-full border cursor-help"
                                style={fundamentalDotStyle(signal)}
                                title={signal}
                                aria-label={signal}
                              />
                            ))}
                          </div>
                        )}
                        <div className="mt-1 flex flex-col items-center gap-0.5">
                          {r.valuation_label && (
                            <span
                              className={`rounded border px-1.5 py-0.5 text-[9px] font-semibold ${valuationClass(r.valuation_label)}`}
                              title={[r.valuation_source, r.valuation_reason].filter(Boolean).join(" | ") || "Current valuation estimate from fundamentals"}
                            >
                              {r.valuation_label}
                            </span>
                          )}
                          {valuationFairValue(r) != null && (
                            <span
                              className={`rounded border px-1.5 py-0.5 text-[9px] font-mono ${
                                (valuationUpsidePct(r) ?? 0) >= 0
                                  ? "border-green/30 bg-green/10 text-green"
                                  : "border-red/30 bg-red/10 text-red"
                              }`}
                              title={[r.valuation_source || "Score-implied fair value", r.valuation_reason].filter(Boolean).join(" | ")}
                            >
                              FV {fmtMoney(valuationFairValue(r))} {fmtSignedPct(valuationUpsidePct(r))}
                            </span>
                          )}
                          <a
                            href={`https://finance.yahoo.com/quote/${encodeURIComponent(r.ticker)}/financials/`}
                            target="_blank"
                            rel="noreferrer"
                            className="text-[9px] font-mono text-accent hover:text-white underline-offset-2 hover:underline"
                            title={`${r.ticker} Yahoo financials`}
                          >
                            Financials
                          </a>
                        </div>
                      </td>
                      <td className="px-2 py-2 text-left text-[10px] whitespace-nowrap border-l border-border/30">
                        <div className="flex flex-col gap-0.5 font-mono leading-tight">
                          {r.swing_spring && (
                            <span className="inline-flex max-w-[130px] items-center gap-1 whitespace-normal text-green">
                              <SpringMarker title={r.swing_spring_text} />
                              Daily spring
                            </span>
                          )}
                          {r.lre_takeaway && (
                            <span className={`max-w-[130px] whitespace-normal ${
                              r.lre_takeaway.includes("bounce risk") || r.lre_takeaway.includes("fade risk")
                                ? "text-yellow"
                                : r.lre_direction === "LONG"
                                  ? "text-green/80"
                                  : r.lre_direction === "SHORT"
                                    ? "text-red/80"
                                    : "text-muted"
                            }`}>
                              {r.lre_takeaway}
                            </span>
                          )}
                          <div className="grid grid-cols-[34px_64px] gap-x-1 gap-y-0.5">
                            <span className="text-muted">Entry</span><span className="text-right text-accent">{fmtMoney(r.entry)}</span>
                            <span className="text-muted">Stop</span><span className="text-right text-red">{fmtMoney(r.stop_loss)}</span>
                            <span className="text-muted">T1</span><span className="text-right text-green">{fmtMoney(r.target1)}</span>
                            <span className="text-muted">Reward</span><span className="text-right text-green">{rewardPct(r.entry, r.target1)}</span>
                            <span className="text-muted">Risk</span><span className="text-right text-muted">{r.risk_pct ? `${r.risk_pct}%` : "—"}</span>
                          </div>
                        </div>
                      </td>
                      <td
                        className="px-2 py-2 text-left text-[10px] whitespace-nowrap border-l border-border/30"
                        title={[r.cpr_interpretation, r.day_spring_text, r.cpr_day_15m_volume_text, r.cpr_day_volume_text, r.cpr_day_ref].filter(Boolean).join(" | ") || undefined}
                      >
                        {r.cpr_day_result ? (
                          <div className="flex flex-col gap-0.5 leading-tight font-mono">
                            <span className={`inline-flex max-w-[170px] items-start gap-1 whitespace-normal ${
                                r.cpr_position === "Above" ? "text-green" :
                                r.cpr_position === "Below" ? "text-red"   :
                                                              "text-yellow"
                              }`}
                            >
                              {r.day_spring && <SpringMarker title={r.day_spring_text} />}
                              {r.cpr_interpretation ?? compactDayResult(r.cpr_day_result)}
                            </span>
                            <div className="grid grid-cols-[38px_64px] gap-x-1 gap-y-0.5">
                              <span className="text-muted">Type</span><span className="text-right text-muted/70">{r.cpr_type}</span>
                              <span className="text-muted">Entry</span><span className="text-right text-accent">{fmtMoney(r.cpr_day_entry)}</span>
                              <span className="text-muted">Stop</span><span className="text-right text-red">{fmtMoney(r.cpr_day_stop)}</span>
                              <span className="text-muted">T1</span><span className="text-right text-green">{fmtMoney(r.cpr_day_t1)}</span>
                              <span className="text-muted">Reward</span><span className="text-right text-green">{rewardPct(r.cpr_day_entry, r.cpr_day_t1)}</span>
                            </div>
                            {(r.cpr_day_trigger_text || r.cpr_day_invalidation_text || r.cpr_day_target_text || r.cpr_day_15m_volume_text) && (
                              <div className="mt-1 border-t border-border/30 pt-1">
                                <div className="grid grid-cols-[34px_96px] gap-x-1 gap-y-0.5">
                                  <span className="text-yellow">V2</span><span className="text-yellow whitespace-normal">Trigger</span>
                                  <span className="text-muted">Trig</span><span className="text-right text-accent whitespace-normal">{r.cpr_day_trigger_text ?? "-"}</span>
                                  <span className="text-muted">Inv</span><span className="text-right text-red whitespace-normal">{r.cpr_day_invalidation_text ?? "-"}</span>
                                  <span className="text-muted">Tgt</span><span className="text-right text-green whitespace-normal">{r.cpr_day_target_text ?? "-"}</span>
                                  <span className="text-muted">15m</span><span className={`text-right whitespace-normal ${dayVolumeColor(r.cpr_day_15m_volume_text)}`}>{r.cpr_day_15m_volume_text ?? "15m pending"}</span>
                                </div>
                              </div>
                            )}
                          </div>
                        ) : <span className="text-muted/40">—</span>}
                      </td>
                      <td
                        className="px-2 py-2 text-left text-[10px] whitespace-nowrap border-l border-border/30"
                        title={r.next_day_summary ?? r.next_day_prediction ?? undefined}
                      >
                        {(r.next_day_outcome || r.next_day_bias) ? (
                          <div className="flex flex-col gap-0.5 leading-tight font-mono">
                            <span className={`max-w-[165px] whitespace-normal font-semibold ${nextDayColor(r.next_day_outcome ?? r.next_day_bias)}`}>
                              {r.next_day_outcome ?? r.next_day_bias}
                            </span>
                            {r.next_day_bias && r.next_day_outcome && (
                              <span className="max-w-[165px] whitespace-normal text-[9px] text-muted">
                                {r.next_day_bias}
                              </span>
                            )}
                            {r.next_day_date && (
                              <span className="text-[9px] text-muted">{r.next_day_date}</span>
                            )}
                            <div className="grid grid-cols-[42px_72px] gap-x-1 gap-y-0.5">
                              <span className="text-muted">ATR</span><span className="text-right text-accent">{fmtMoney(r.next_day_atr)}</span>
                              <span className="text-muted">Up &gt;</span><span className="text-right text-green">{fmtMoney(r.next_day_trigger_up)}</span>
                              <span className="text-muted">Dn &lt;</span><span className="text-right text-red">{fmtMoney(r.next_day_trigger_down)}</span>
                              <span className="text-muted">Pivot</span><span className="text-right text-muted/80">{fmtMoney(r.next_day_pivot)}</span>
                              <span className="text-muted">Ref</span><span className="text-right text-muted/80 truncate">{r.next_day_ref ?? "N/A"}</span>
                              <span className="text-muted">Target</span><span className={`text-right ${nextDayColor(r.next_day_bias)}`}>{fmtMoney(r.next_day_target)}</span>
                            </div>
                          </div>
                        ) : <span className="text-muted/40">N/A</span>}
                      </td>
                      <td className="w-[42px] px-1.5 py-2 text-right font-mono text-xs whitespace-nowrap">
                        {r.short_pct != null
                          ? <span className={r.short_pct >= 20 ? "text-red font-bold" : r.short_pct >= 10 ? "text-yellow" : "text-muted"}>
                              {Math.round(r.short_pct)}%
                            </span>
                          : <span className="text-muted">—</span>}
                      </td>
                      <td className="px-3 py-2.5 text-left whitespace-nowrap">
                        <div className="flex flex-col gap-1.5">
                          <div
                            onDoubleClick={() => (r.opt_summary || r.opt_alt) && setOptModal({ r })}
                            title={(r.opt_summary || r.opt_alt) ? "Double-click to view & copy" : undefined}
                          >
                        {optEmoji && optShort ? (
                          <span className="flex items-center gap-1 cursor-pointer select-none group">
                            <span className={`text-[9px] font-bold px-1 py-0.5 rounded border ${
                              r.opt_source === "alpaca"
                                ? "bg-accent/10 text-accent border-accent/30"
                                : "bg-muted/10 text-muted border-border"
                            }`}>
                              {r.opt_source === "alpaca" ? "A" : "Y"}
                            </span>
                            <span className="text-xs font-mono">
                              {optEmoji}{" "}
                              <span className={
                                r.direction === "LONG"  ? "text-green" :
                                r.direction === "SHORT" ? "text-red"   : "text-accent"
                              }>{optShort}</span>
                              {r.opt_debit != null && (
                                <span className="text-muted ml-1">${r.opt_debit}</span>
                              )}
                              {r.opt_profit != null && (
                                <span className="text-green ml-1">→${r.opt_profit}</span>
                              )}
                              {hasZebra && (
                                <span className="text-accent ml-1">Z</span>
                              )}
                              {hasButterflyAlt && (
                                <span className="text-yellow ml-1">Fly</span>
                              )}
                            </span>
                            <span className="text-muted/40 text-[10px] opacity-0 group-hover:opacity-100 transition-opacity">⤢</span>
                          </span>
                        ) : (
                          <span className="text-muted text-xs">—</span>
                        )}
                          </div>

                          {hasOtmData && (
                            <div
                              className="cursor-pointer select-none"
                              onDoubleClick={() => setOtmModal({ r })}
                              title={otmInterp || "Double-click to view all OTM contracts"}
                            >
                              <span className="flex flex-col gap-0.5">
                                {[topCall, topPut].filter(Boolean).map((c, i) => {
                                  const cc = c!.type === "CALL" ? "text-green" : "text-red";
                                  return (
                                    <span key={i} className={`flex items-center gap-1 text-[10px] font-mono ${c!.unusual ? "bg-yellow/5 rounded px-0.5" : ""}`}>
                                      {c!.unusual && <span className="text-yellow text-[8px]">⚡</span>}
                                      <span className={`font-bold ${cc}`}>{c!.type[0]}</span>
                                      <span className="text-white">${c!.strike}</span>
                                      <span className="text-muted">{c!.otm_pct}%otm</span>
                                      <span className={c!.vol_oi_ratio > 0.5 ? "text-yellow" : "text-muted"}>
                                        {c!.volume >= 1000 ? `${(c!.volume / 1000).toFixed(1)}K` : c!.volume}v
                                      </span>
                                    </span>
                                  );
                                })}
                              </span>
                            </div>
                          )}
                        </div>
                      </td>

                      {/* OTM Liquid column */}
                      {(() => {
                        const topCall    = r.opt_liquid?.find(c => c.type === "CALL");
                        const topPut     = r.opt_liquid?.find(c => c.type === "PUT");
                        const hasData    = topCall || topPut;
                        const interp     = hasData ? interpretOtmFlow(r.opt_liquid ?? []) : "";
                        return (
                          <td
                            className="hidden"
                            onDoubleClick={() => hasData && setOtmModal({ r })}
                            title={interp || (hasData ? "Double-click to view all OTM contracts" : undefined)}
                          >
                            {hasData ? (
                              <span className="flex flex-col gap-0.5 cursor-pointer select-none group">
                                {[topCall, topPut].filter(Boolean).map((c, i) => {
                                  const cc = c!.type === "CALL" ? "text-green" : "text-red";
                                  return (
                                    <span key={i} className={`flex items-center gap-1 text-[10px] font-mono ${c!.unusual ? "bg-yellow/5 rounded px-0.5" : ""}`}>
                                      {c!.unusual && <span className="text-yellow text-[8px]">⚡</span>}
                                      <span className={`font-bold ${cc}`}>{c!.type[0]}</span>
                                      <span className="text-white">${c!.strike}</span>
                                      <span className="text-muted">{c!.otm_pct}%otm</span>
                                      <span className={c!.vol_oi_ratio > 0.5 ? "text-yellow" : "text-muted"}>
                                        {c!.volume >= 1000 ? `${(c!.volume/1000).toFixed(1)}K` : c!.volume}v
                                      </span>
                                    </span>
                                  );
                                })}
                                <span className="text-muted/40 text-[9px] opacity-0 group-hover:opacity-100 transition-opacity">⤢ details</span>
                              </span>
                            ) : (
                              <span className="text-muted text-xs">—</span>
                            )}
                          </td>
                        );
                      })()}

                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ── Scanning skeleton ── */}
      {scanning && filtered.length === 0 && (
        <div className="card flex flex-col items-center justify-center py-16 gap-3">
          <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          <p className="text-muted text-sm">Scanning {progress.total} stocks…</p>
        </div>
      )}

      {/* ── Empty state ── */}
      {!scanning && results.length > 0 && filtered.length === 0 && (
        <div className="card flex items-center justify-center py-12">
          <p className="text-muted text-sm">No stocks match the current filter.</p>
        </div>
      )}

      {/* ── Options summary modal ── */}
      {optModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => setOptModal(null)}
        >
          <div
            className="bg-card border border-border rounded-xl shadow-2xl p-5 w-full max-w-lg mx-4 space-y-3"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold text-white">{optModal.r.ticker} — Options Play</span>
              <button onClick={() => setOptModal(null)} className="text-muted hover:text-white text-lg leading-none">×</button>
            </div>
            {optModal.r.opt_quote_ts && (
              <p className="text-[11px] text-muted font-mono">
                Quote: {(() => {
                  try {
                    const d = new Date(optModal.r.opt_quote_ts!);
                    return d.toLocaleString("en-US", {
                      timeZone: "America/New_York",
                      month: "short", day: "numeric",
                      hour: "2-digit", minute: "2-digit",
                      hour12: true,
                    }) + " ET";
                  } catch { return optModal.r.opt_quote_ts; }
                })()}
              </p>
            )}
            <textarea
              readOnly
              autoFocus
              onFocus={e => e.target.select()}
              value={[optModal.r.opt_summary, optModal.r.opt_alt].filter(Boolean).join("\n\n")}
              rows={7}
              className="w-full bg-surface border border-border rounded-lg p-3 text-sm font-mono text-white resize-none focus:outline-none focus:border-accent"
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => copyText([optModal.r.opt_summary, optModal.r.opt_alt].filter(Boolean).join("\n\n"))}
                className="px-4 py-1.5 text-xs font-semibold rounded-lg bg-accent text-black hover:bg-accent/80 transition-colors"
              >
                {copied ? "✓ Copied" : "Copy"}
              </button>
              <button
                onClick={() => setOptModal(null)}
                className="px-4 py-1.5 text-xs font-semibold rounded-lg border border-border text-muted hover:text-white transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── OTM Liquid modal ── */}
      {otmModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => setOtmModal(null)}
        >
          <div
            className="bg-card border border-border rounded-xl shadow-2xl p-5 w-full max-w-2xl mx-4 space-y-3"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold text-white">{otmModal.r.ticker} — OTM Liquid Options</span>
              <button onClick={() => setOtmModal(null)} className="text-muted hover:text-white text-lg leading-none">×</button>
            </div>
            {otmModal.r.opt_liquid && otmModal.r.opt_liquid.length > 0 && (
              <p className="text-xs text-accent bg-accent/5 border border-accent/20 rounded px-2 py-1.5">
                {interpretOtmFlow(otmModal.r.opt_liquid)}
              </p>
            )}
            <div className="space-y-1">
              <div className="grid grid-cols-[auto_auto_auto_auto_auto_auto_auto] text-[10px] text-muted px-2 pb-1 border-b border-border gap-x-3">
                <span>Type</span>
                <span className="text-right">Strike</span>
                <span className="text-right">Exp</span>
                <span className="text-right">OTM%</span>
                <span className="text-right">Volume</span>
                <span className="text-right">OI</span>
                <span className="text-right">IV%</span>
              </div>
              {(otmModal.r.opt_liquid ?? []).map((c, i) => {
                const cc = c.type === "CALL" ? "text-green" : "text-red";
                return (
                  <div key={i} className={`grid grid-cols-[auto_auto_auto_auto_auto_auto_auto] text-[12px] font-mono px-2 py-1.5 rounded gap-x-3 ${c.unusual ? "bg-yellow/5" : "hover:bg-surface/50"}`}>
                    <span className={`font-bold ${cc} flex items-center gap-1`}>
                      {c.type}
                      {c.unusual && <span className="text-[9px] bg-yellow/20 text-yellow px-1 rounded">⚡</span>}
                    </span>
                    <span className={`text-right ${cc}`}>${c.strike}</span>
                    <span className="text-right text-muted">{c.expiry.slice(5)}</span>
                    <span className="text-right text-muted">{c.otm_pct}%</span>
                    <span className={`text-right ${c.vol_oi_ratio > 0.5 ? "text-yellow font-semibold" : "text-white"}`}>
                      {c.volume >= 1000 ? `${(c.volume/1000).toFixed(1)}K` : c.volume}
                    </span>
                    <span className="text-right text-muted">
                      {c.oi >= 1000 ? `${(c.oi/1000).toFixed(1)}K` : c.oi}
                    </span>
                    <span className={`text-right ${c.iv > 80 ? "text-red" : c.iv > 50 ? "text-yellow" : "text-muted"}`}>
                      {c.iv}%
                    </span>
                  </div>
                );
              })}
            </div>
            <p className="text-[10px] text-muted/60">⚡ unusual flow &nbsp;·&nbsp; Vol highlighted when Vol/OI &gt; 0.5</p>
            <div className="flex justify-end">
              <button
                onClick={() => setOtmModal(null)}
                className="px-4 py-1.5 text-xs font-semibold rounded-lg border border-border text-muted hover:text-white transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
