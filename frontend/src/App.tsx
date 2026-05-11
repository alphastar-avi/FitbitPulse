import { useState, useEffect, useRef } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Activity, Heart, Moon, Loader2, Download, Wind, Thermometer, Brain, Droplets, Database, RefreshCw, Flame, Navigation, Clock } from "lucide-react"
import { Bar, BarChart, CartesianGrid, XAxis, Tooltip, ResponsiveContainer, LineChart, Line, YAxis, AreaChart, Area } from "recharts"

const API_BASE = "/api/raw"

const ttStyle = { contentStyle: { backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }, cursor: { fill: 'hsl(var(--muted))' } }
const ax = { stroke: "hsl(var(--muted-foreground))", fontSize: 11, tickLine: false, axisLine: false }

const ENDPOINTS = [
  { type: 'steps', label: 'Daily Steps', description: 'Aggregated daily step count using dailyRollUp.', icon: '👣' },
  { type: 'daily-resting-heart-rate', label: 'Resting Heart Rate', description: 'Lowest heart rate recorded during sleep.', icon: '❤️' },
  { type: 'sleep', label: 'Sleep Sessions', description: 'Full sleep session data including stages and total duration.', icon: '🌙' },
  { type: 'daily-heart-rate-variability', label: 'Heart Rate Variability', description: 'Variation in time between heartbeats. Includes entropy and RMSSD.', icon: '🧠' },
  { type: 'daily-respiratory-rate', label: 'Breathing Rate', description: 'Average breaths per minute measured during sleep.', icon: '💨' },
  { type: 'daily-sleep-temperature-derivations', label: 'Wrist Skin Temperature', description: 'Nightly wrist temperature vs 30-day baseline.', icon: '🌡️' },
  { type: 'daily-oxygen-saturation', label: 'Blood Oxygen (SpO2)', description: 'Estimated blood oxygen saturation measured during sleep.', icon: '🩸' },
  { type: 'active-zone-minutes', label: 'Active Zone Minutes', description: 'Minutes spent in fat-burn, cardio, or peak heart rate zones.', icon: '⚡' },
  { type: 'active-minutes', label: 'Active Minutes', description: 'Individual minutes of physical activity.', icon: '🏃' },
  { type: 'distance', label: 'Distance', description: 'Distance traveled in meters.', icon: '📍' },
  { type: 'sedentary-period', label: 'Sedentary Periods', description: 'Periods of inactivity.', icon: '🪑' },
]

function MiniChart({ data, dataKey, color, type = 'bar' }: any) {
  // Only show the last 30 items for the UI
  const displayData = data.slice(-30)
  return (
    <div className="h-[100px] mt-2">
      <ResponsiveContainer width="100%" height="100%">
        {type === 'line' ? (
          <LineChart data={displayData} margin={{ top: 2, right: 4, left: -30, bottom: 0 }}>
            <XAxis dataKey="date" {...ax} tick={false} /><YAxis {...ax} domain={['dataMin - 2', 'dataMax + 2']} />
            <Tooltip {...ttStyle} /><Line type="monotone" dataKey={dataKey} stroke={color} strokeWidth={2} dot={false} />
          </LineChart>
        ) : type === 'area' ? (
          <AreaChart data={displayData} margin={{ top: 2, right: 4, left: -30, bottom: 0 }}>
            <XAxis dataKey="date" {...ax} tick={false} /><YAxis {...ax} />
            <Tooltip {...ttStyle} /><Area type="monotone" dataKey={dataKey} stroke={color} fillOpacity={0.3} fill={color} />
          </AreaChart>
        ) : (
          <BarChart data={displayData} margin={{ top: 2, right: 4, left: -30, bottom: 0 }}>
            <XAxis dataKey="date" {...ax} tick={false} /><YAxis {...ax} />
            <Tooltip {...ttStyle} /><Bar dataKey={dataKey} fill={color} radius={[3, 3, 0, 0]} />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}

function DataTable({ rows, cols }: { rows: any[], cols: { key: string, label: string, cls?: string }[] }) {
  // Only show the last 30 items for the UI, reversed so newest is on top
  const displayRows = [...rows].slice(-30).reverse()
  return (
    <div className="mt-3 overflow-y-auto max-h-[200px]">
      <table className="w-full text-xs border-collapse">
        <thead className="sticky top-0 bg-card"><tr className="border-b border-border">{cols.map(c => <th key={c.key} className="py-1 px-2 text-left text-muted-foreground font-medium">{c.label}</th>)}</tr></thead>
        <tbody>{displayRows.map((r, i) => <tr key={i} className="border-b border-border hover:bg-muted/30">{cols.map(c => <td key={c.key} className={`py-1 px-2 ${c.cls || 'text-foreground'}`}>{r[c.key]}</td>)}</tr>)}</tbody>
      </table>
    </div>
  )
}

export default function App() {
  const [data, setData] = useState<any>({ steps: [], hr: [], sleep: [], hrv: [], resp: [], wristTemp: [], spo2: [], distance: [], activeMins: [], activeZones: [], sedentary: [] })
  const [rawCache, setRawCache] = useState<Record<string, any>>({})
  const [loadingRaw, setLoadingRaw] = useState<Record<string, boolean>>({})
  const [summary, setSummary] = useState<any>({})
  const [loading, setLoading] = useState(true)
  
  // Refresh & Rate limit state
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [refreshCount, setRefreshCount] = useState(0)
  const [cooldownTime, setCooldownTime] = useState(0)
  const refreshHistory = useRef<number[]>([])

  useEffect(() => {
    if (cooldownTime > 0) {
      const timer = setTimeout(() => setCooldownTime(c => c - 1), 1000)
      return () => clearTimeout(timer)
    }
  }, [cooldownTime])

  const fetchAll = async () => {
    // Rate limit check: max 6 requests per 60 seconds
    const now = Date.now()
    refreshHistory.current = refreshHistory.current.filter(time => now - time < 60000)
    if (refreshHistory.current.length >= 6) {
      const oldest = refreshHistory.current[0]
      const waitTime = Math.ceil((60000 - (now - oldest)) / 1000)
      setCooldownTime(waitTime)
      return
    }
    refreshHistory.current.push(now)
    setRefreshCount(c => c + 1)
    
    setLoading(true)
    try {
      const types = ['steps','daily-resting-heart-rate','sleep','daily-heart-rate-variability','daily-respiratory-rate','daily-sleep-temperature-derivations','daily-oxygen-saturation', 'distance', 'active-minutes', 'active-zone-minutes', 'sedentary-period']
      const results = await Promise.all(types.map(t => fetch(`${API_BASE}?type=${t}`).then(r => r.json()).catch(() => ({}))))
      const [stepsJ, hrJ, sleepJ, hrvJ, respJ, tempJ, spo2J, distJ, actMinsJ, actZoneJ, sedJ] = results

      const formatDt = (y: number, m: number, d: number) => {
        const dt = new Date(y, m - 1, d)
        return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) }
      }
      const formatIso = (isoStr: string) => {
        const dt = new Date(isoStr)
        return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) }
      }

      const steps = (stepsJ.rollupDataPoints || []).filter((d: any) => d.steps?.countSum).map((d: any) => ({ ...formatDt(d.civilStartTime.date.year, d.civilStartTime.date.month, d.civilStartTime.date.day), steps: parseInt(d.steps.countSum) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const hr = (hrJ.dataPoints || []).map((d: any) => ({ ...formatDt(d.dailyRestingHeartRate.date.year, d.dailyRestingHeartRate.date.month, d.dailyRestingHeartRate.date.day), bpm: parseInt(d.dailyRestingHeartRate.beatsPerMinute) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const sleep = (sleepJ.dataPoints || []).filter((d: any) => d.sleep?.summary).map((d: any) => {
        const stages: any[] = d.sleep.summary.stagesSummary || []
        const mins = (type: string) => parseFloat(((parseInt(stages.find((s: any) => s.type === type)?.minutes ?? '0') || 0) / 60).toFixed(1))
        return { ...formatIso(d.sleep.interval.startTime), hours: parseFloat(((parseInt(d.sleep.summary.minutesAsleep ?? '0') || 0) / 60).toFixed(1)), deep: mins('DEEP'), rem: mins('REM'), light: mins('LIGHT'), awake: mins('AWAKE') }
      }).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const hrv = (hrvJ.dataPoints || []).map((d: any) => ({ ...formatDt(d.dailyHeartRateVariability.date.year, d.dailyHeartRateVariability.date.month, d.dailyHeartRateVariability.date.day), hrv: d.dailyHeartRateVariability.averageHeartRateVariabilityMilliseconds ?? 0, entropy: parseFloat(Number(d.dailyHeartRateVariability.entropy || 0).toFixed(2)), rmssd: parseFloat(Number(d.dailyHeartRateVariability.deepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds || 0).toFixed(1)) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const resp = (respJ.dataPoints || []).map((d: any) => ({ ...formatDt(d.dailyRespiratoryRate.date.year, d.dailyRespiratoryRate.date.month, d.dailyRespiratoryRate.date.day), bpm: d.dailyRespiratoryRate.breathsPerMinute })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const wristTemp = (tempJ.dataPoints || []).map((d: any) => {
        const t = d.dailySleepTemperatureDerivations
        const nightly = parseFloat(Number(t.nightlyTemperatureCelsius || 0).toFixed(2))
        const baseline = parseFloat(Number(t.baselineTemperatureCelsius || 0).toFixed(2))
        return { ...formatDt(t.date.year, t.date.month, t.date.day), nightly, baseline, deviation: parseFloat((nightly - baseline).toFixed(2)) }
      }).sort((a: any, b: any) => a.rawDate - b.rawDate)

      const spo2 = (spo2J.dataPoints || []).map((d: any) => ({ ...formatDt(d.dailyOxygenSaturation.date.year, d.dailyOxygenSaturation.date.month, d.dailyOxygenSaturation.date.day), avg: parseFloat(Number(d.dailyOxygenSaturation.averagePercentage || 0).toFixed(1)), low: Number(d.dailyOxygenSaturation.lowerBoundPercentage || 0), high: Number(d.dailyOxygenSaturation.upperBoundPercentage || 0) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      // Activity: distance (meters)
      const distMap = new Map();
      (distJ.dataPoints || []).forEach((d: any) => {
        if (!d.distance?.interval?.civilStartTime) return
        const { date, rawDate } = formatDt(d.distance.interval.civilStartTime.date.year, d.distance.interval.civilStartTime.date.month, d.distance.interval.civilStartTime.date.day)
        const meters = parseFloat(d.distance.millimeters || '0') / 1000
        distMap.set(date, { date, rawDate, meters: (distMap.get(date)?.meters || 0) + meters })
      })
      const distance = Array.from(distMap.values()).map(d => ({ ...d, meters: Math.round(d.meters) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      // Activity: active minutes
      const actMap = new Map();
      (actMinsJ.dataPoints || []).forEach((d: any) => {
        if (!d.activeMinutes?.interval?.civilStartTime) return
        const { date, rawDate } = formatDt(d.activeMinutes.interval.civilStartTime.date.year, d.activeMinutes.interval.civilStartTime.date.month, d.activeMinutes.interval.civilStartTime.date.day)
        let mins = 0
        ;(d.activeMinutes.activeMinutesByActivityLevel || []).forEach((a: any) => mins += parseInt(a.activeMinutes || '0'))
        actMap.set(date, { date, rawDate, minutes: (actMap.get(date)?.minutes || 0) + mins })
      })
      const activeMins = Array.from(actMap.values()).sort((a: any, b: any) => a.rawDate - b.rawDate)

      // Activity: active zone minutes
      const actZoneMap = new Map();
      (actZoneJ.dataPoints || []).forEach((d: any) => {
        if (!d.activeZoneMinutes?.interval?.civilStartTime) return
        const { date, rawDate } = formatDt(d.activeZoneMinutes.interval.civilStartTime.date.year, d.activeZoneMinutes.interval.civilStartTime.date.month, d.activeZoneMinutes.interval.civilStartTime.date.day)
        const mins = parseInt(d.activeZoneMinutes.activeZoneMinutes || '0')
        const zone = d.activeZoneMinutes.heartRateZone
        const ex = actZoneMap.get(date) || { date, rawDate, fatBurn: 0, cardio: 0, peak: 0 }
        if (zone === 'FAT_BURN') ex.fatBurn += mins
        else if (zone === 'CARDIO') ex.cardio += mins
        else if (zone === 'PEAK') ex.peak += mins
        actZoneMap.set(date, ex)
      })
      const activeZones = Array.from(actZoneMap.values()).sort((a: any, b: any) => a.rawDate - b.rawDate)

      // Sedentary periods
      const sedMap = new Map();
      (sedJ.dataPoints || []).forEach((d: any) => {
        const start = new Date(d.sedentaryPeriod.interval.startTime)
        const end = new Date(d.sedentaryPeriod.interval.endTime)
        const mins = (end.getTime() - start.getTime()) / 60000
        const { date, rawDate } = formatIso(d.sedentaryPeriod.interval.startTime)
        sedMap.set(date, { date, rawDate, minutes: (sedMap.get(date)?.minutes || 0) + mins })
      })
      const sedentary = Array.from(sedMap.values()).map(d => ({ ...d, hours: parseFloat((d.minutes / 60).toFixed(1)) })).sort((a: any, b: any) => a.rawDate - b.rawDate)

      setData({ steps, hr, sleep, hrv, resp, wristTemp, spo2, distance, activeMins, activeZones, sedentary })

      const avg = (arr: any[], key: string) => arr.length ? Math.round(arr.reduce((s, d) => s + d[key], 0) / arr.length * 10) / 10 : 0
      setSummary({ 
        avgSteps: avg(steps, 'steps'), avgHr: avg(hr, 'bpm'), avgSleep: avg(sleep, 'hours'), avgHrv: avg(hrv, 'hrv'), 
        avgResp: avg(resp, 'bpm'), avgTemp: avg(wristTemp, 'nightly'), avgSpo2: avg(spo2, 'avg'), avgDist: avg(distance, 'meters'),
        avgAct: avg(activeMins, 'minutes'), avgSed: avg(sedentary, 'hours')
      })
      setLastUpdated(new Date())
    } catch (e) { console.error('Fetch error:', e) }
    finally { setLoading(false) }
  }

  useEffect(() => { fetchAll() }, [])

  const fetchRaw = async (type: string) => {
    if (rawCache[type]) return
    setLoadingRaw(p => ({ ...p, [type]: true }))
    try {
      const res = await fetch(`${API_BASE}?type=${type}`)
      const json = await res.json()
      setRawCache(p => ({ ...p, [type]: json }))
    } catch (e) { console.error(e) }
    finally { setLoadingRaw(p => ({ ...p, [type]: false })) }
  }

  const downloadJSON = (type: string) => {
    const blob = new Blob([JSON.stringify(rawCache[type], null, 2)], { type: 'application/json' })
    const link = document.createElement('a'); link.href = URL.createObjectURL(blob)
    link.download = `fitbit_charge6_${type}_${new Date().toISOString().split('T')[0]}.json`; link.click()
  }

  const downloadAllJSON = async () => {
    const allData: Record<string, any> = {}
    for (const ep of ENDPOINTS) {
      try { allData[ep.type] = await fetch(`${API_BASE}?type=${ep.type}`).then(r => r.json()) } catch {}
    }
    const blob = new Blob([JSON.stringify(allData, null, 2)], { type: 'application/json' })
    const link = document.createElement('a'); link.href = URL.createObjectURL(blob)
    link.download = `fitbit_charge6_all_raw_${new Date().toISOString().split('T')[0]}.json`; link.click()
  }

  const downloadCSV = () => {
    const map = new Map<string, any>()
    const merge = (date: string, obj: any) => map.set(date, { ...map.get(date), Date: date, ...obj })
    data.steps.forEach((d: any) => merge(d.date, { Steps: d.steps }))
    data.hr.forEach((d: any) => merge(d.date, { RestingHR_BPM: d.bpm }))
    data.sleep.forEach((d: any) => merge(d.date, { Sleep_Hours: d.hours, Deep_Hours: d.deep, REM_Hours: d.rem, Light_Hours: d.light, Awake_Hours: d.awake }))
    data.hrv.forEach((d: any) => merge(d.date, { HRV_ms: d.hrv, HRV_Entropy: d.entropy, HRV_RMSSD_ms: d.rmssd }))
    data.resp.forEach((d: any) => merge(d.date, { BreathingRate_BPM: d.bpm }))
    data.wristTemp.forEach((d: any) => merge(d.date, { WristTemp_Nightly_C: d.nightly, WristTemp_Baseline_C: d.baseline, WristTemp_Deviation_C: d.deviation }))
    data.spo2.forEach((d: any) => merge(d.date, { SpO2_Avg_Pct: d.avg, SpO2_Low_Pct: d.low, SpO2_High_Pct: d.high }))
    data.distance.forEach((d: any) => merge(d.date, { Distance_Meters: d.meters }))
    data.activeMins.forEach((d: any) => merge(d.date, { Active_Minutes: d.minutes }))
    data.activeZones.forEach((d: any) => merge(d.date, { Zone_FatBurn_Mins: d.fatBurn, Zone_Cardio_Mins: d.cardio, Zone_Peak_Mins: d.peak }))
    data.sedentary.forEach((d: any) => merge(d.date, { Sedentary_Hours: d.hours }))
    
    const rows = Array.from(map.values())
    const headers = Array.from(new Set(rows.flatMap(Object.keys)))
    const csv = [headers.join(','), ...rows.map(r => headers.map(h => r[h] ?? '').join(','))].join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const link = document.createElement('a'); link.href = URL.createObjectURL(blob)
    link.download = `fitbit_charge6_${new Date().toISOString().split('T')[0]}.csv`; link.click()
  }

  if (loading && !lastUpdated) return <div className="flex h-screen items-center justify-center bg-background"><Loader2 className="h-8 w-8 animate-spin text-emerald-500" /></div>

  return (
    <div className="min-h-screen bg-background text-foreground p-6 font-sans pb-24">
      <div className="max-w-6xl mx-auto space-y-6">

        <header className="flex flex-col md:flex-row md:items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-medium bg-emerald-500/20 text-emerald-400 px-2 py-0.5 rounded-full">FITBIT PLATFORM</span>
              <span className="text-xs text-muted-foreground">Charge 6 · Last 90 days available</span>
            </div>
            <h1 className="text-3xl font-bold tracking-tight">Fitbit Charge 6 <span className="text-emerald-500">Health Dashboard</span></h1>
            <p className="text-sm text-muted-foreground mt-1">All metrics sourced from your Fitbit Charge 6 via Google Health API</p>
          </div>
          <div className="flex flex-col items-end gap-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              {lastUpdated && <span>Last updated: {lastUpdated.toLocaleTimeString()}</span>}
              <Button onClick={fetchAll} disabled={cooldownTime > 0 || loading} variant="outline" size="sm" className="h-7 px-2">
                {loading ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3 mr-1" />}
                {cooldownTime > 0 ? `Wait ${cooldownTime}s` : 'Refresh'}
              </Button>
            </div>
            <div className="flex gap-2">
              <Button onClick={downloadCSV} variant="outline" size="sm" className="gap-2 h-8 text-xs"><Download className="h-3 w-3" />Export Full CSV (90d)</Button>
              <Button onClick={downloadAllJSON} variant="outline" size="sm" className="gap-2 h-8 text-xs"><Database className="h-3 w-3" />Export All JSON (90d)</Button>
            </div>
          </div>
        </header>

        {/* Summary Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-3">
          {[
            { label: 'Steps/day', value: summary.avgSteps.toLocaleString(), color: 'text-emerald-400' },
            { label: 'Active Mins', value: `${summary.avgAct}m`, color: 'text-orange-400' },
            { label: 'Distance', value: `${(summary.avgDist/1000).toFixed(1)}km`, color: 'text-yellow-400' },
            { label: 'Resting HR', value: `${summary.avgHr} bpm`, color: 'text-rose-400' },
            { label: 'Sleep', value: `${summary.avgSleep}h`, color: 'text-violet-400' },
            { label: 'HRV', value: `${summary.avgHrv}ms`, color: 'text-blue-400' },
            { label: 'SpO2', value: `${summary.avgSpo2}%`, color: 'text-sky-400' },
            { label: 'Sedentary', value: `${summary.avgSed}h`, color: 'text-gray-400' },
          ].map(s => (
            <Card key={s.label} className="p-3">
              <p className="text-xs text-muted-foreground whitespace-nowrap">{s.label}</p>
              <p className={`text-lg font-bold mt-0.5 ${s.color}`}>{s.value}</p>
            </Card>
          ))}
        </div>

        <Tabs defaultValue="overview">
          
          <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50">
            <div className="bg-background/80 backdrop-blur-md border border-border rounded-full p-1 shadow-lg">
              <TabsList className="bg-transparent border-none h-auto p-0 gap-1 flex">
                <TabsTrigger value="overview" className="rounded-full px-4 py-2 data-[state=active]:bg-emerald-500/20 data-[state=active]:text-emerald-400"><Activity className="h-4 w-4 mr-2" />Overview</TabsTrigger>
                <TabsTrigger value="activity" className="rounded-full px-4 py-2 data-[state=active]:bg-orange-500/20 data-[state=active]:text-orange-400"><Flame className="h-4 w-4 mr-2" />Activity</TabsTrigger>
                <TabsTrigger value="recovery" className="rounded-full px-4 py-2 data-[state=active]:bg-blue-500/20 data-[state=active]:text-blue-400"><Brain className="h-4 w-4 mr-2" />Recovery</TabsTrigger>
                <TabsTrigger value="sleep" className="rounded-full px-4 py-2 data-[state=active]:bg-violet-500/20 data-[state=active]:text-violet-400"><Moon className="h-4 w-4 mr-2" />Sleep</TabsTrigger>
                <TabsTrigger value="raw" className="rounded-full px-4 py-2 data-[state=active]:bg-muted"><Database className="h-4 w-4 mr-2" />Raw Data</TabsTrigger>
              </TabsList>
            </div>
          </div>

          {/* OVERVIEW */}
          <TabsContent value="overview" className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mt-4">
            {[
              { title: 'Daily Steps', icon: <Activity className="h-4 w-4 text-emerald-500" />, chart: <MiniChart data={data.steps} dataKey="steps" color="#10b981" />, rows: data.steps, cols: [{ key: 'date', label: 'Date' }, { key: 'steps', label: 'Steps', cls: 'text-emerald-400 font-medium' }] },
              { title: 'Resting Heart Rate', icon: <Heart className="h-4 w-4 text-rose-500" />, chart: <MiniChart data={data.hr} dataKey="bpm" color="#f43f5e" type="line" />, rows: data.hr, cols: [{ key: 'date', label: 'Date' }, { key: 'bpm', label: 'BPM', cls: 'text-rose-400 font-medium' }] },
              { title: 'Sleep Duration', icon: <Moon className="h-4 w-4 text-violet-500" />, chart: <MiniChart data={data.sleep} dataKey="hours" color="#a855f7" />, rows: data.sleep, cols: [{ key: 'date', label: 'Date' }, { key: 'hours', label: 'Hours', cls: 'text-violet-400 font-medium' }] },
              { title: 'Blood Oxygen (SpO2)', icon: <Droplets className="h-4 w-4 text-sky-500" />, chart: <MiniChart data={data.spo2} dataKey="avg" color="#0ea5e9" type="line" />, rows: data.spo2, cols: [{ key: 'date', label: 'Date' }, { key: 'avg', label: 'Avg%', cls: 'text-sky-400 font-medium' }, { key: 'low', label: 'Low' }, { key: 'high', label: 'High' }] },
            ].map(card => (
              <Card key={card.title}>
                <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">{card.title}</CardTitle>{card.icon}</CardHeader>
                <CardContent>{card.chart}<DataTable rows={card.rows} cols={card.cols} /></CardContent>
              </Card>
            ))}
          </TabsContent>

          {/* ACTIVITY */}
          <TabsContent value="activity" className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">Distance (Meters)</CardTitle><Navigation className="h-4 w-4 text-yellow-500" /></CardHeader>
              <CardContent>
                <MiniChart data={data.distance} dataKey="meters" color="#eab308" type="area" />
                <DataTable rows={data.distance} cols={[{ key: 'date', label: 'Date' }, { key: 'meters', label: 'Meters', cls: 'text-yellow-400 font-medium' }]} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">Active Minutes</CardTitle><Flame className="h-4 w-4 text-orange-500" /></CardHeader>
              <CardContent>
                <MiniChart data={data.activeMins} dataKey="minutes" color="#f97316" />
                <DataTable rows={data.activeMins} cols={[{ key: 'date', label: 'Date' }, { key: 'minutes', label: 'Minutes', cls: 'text-orange-400 font-medium' }]} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">Sedentary Hours</CardTitle><Clock className="h-4 w-4 text-gray-400" /></CardHeader>
              <CardContent>
                <MiniChart data={data.sedentary} dataKey="hours" color="#9ca3af" />
                <DataTable rows={data.sedentary} cols={[{ key: 'date', label: 'Date' }, { key: 'hours', label: 'Hours', cls: 'text-gray-400 font-medium' }]} />
              </CardContent>
            </Card>
            <Card className="md:col-span-3">
              <CardHeader><CardTitle>Daily Heart Rate Zones (Minutes)</CardTitle></CardHeader>
              <CardContent>
                <div className="h-[280px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={data.activeZones.slice(-30)} margin={{ left: -20, right: 8, top: 4 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                      <XAxis dataKey="date" {...ax} /><YAxis {...ax} />
                      <Tooltip contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }} />
                      <Bar dataKey="fatBurn" stackId="a" fill="#facc15" name="Fat Burn" />
                      <Bar dataKey="cardio" stackId="a" fill="#fb923c" name="Cardio" />
                      <Bar dataKey="peak" stackId="a" fill="#ef4444" name="Peak" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
                <DataTable rows={data.activeZones} cols={[{ key: 'date', label: 'Date' }, { key: 'fatBurn', label: 'Fat Burn', cls: 'text-yellow-400' }, { key: 'cardio', label: 'Cardio', cls: 'text-orange-400' }, { key: 'peak', label: 'Peak', cls: 'text-red-400' }]} />
              </CardContent>
            </Card>
          </TabsContent>

          {/* RECOVERY */}
          <TabsContent value="recovery" className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">HRV</CardTitle><Brain className="h-4 w-4 text-blue-500" /></CardHeader>
              <CardContent>
                <MiniChart data={data.hrv} dataKey="hrv" color="#3b82f6" type="line" />
                <DataTable rows={data.hrv} cols={[{ key: 'date', label: 'Date' }, { key: 'hrv', label: 'HRV ms', cls: 'text-blue-400 font-medium' }, { key: 'rmssd', label: 'RMSSD' }, { key: 'entropy', label: 'Entropy' }]} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">Breathing Rate</CardTitle><Wind className="h-4 w-4 text-cyan-500" /></CardHeader>
              <CardContent>
                <MiniChart data={data.resp} dataKey="bpm" color="#06b6d4" type="line" />
                <DataTable rows={data.resp} cols={[{ key: 'date', label: 'Date' }, { key: 'bpm', label: 'Breaths/min', cls: 'text-cyan-400 font-medium' }]} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between"><CardTitle className="text-sm">Wrist Temperature</CardTitle><Thermometer className="h-4 w-4 text-orange-500" /></CardHeader>
              <CardContent>
                <MiniChart data={data.wristTemp} dataKey="deviation" color="#f97316" type="line" />
                <DataTable rows={data.wristTemp} cols={[{ key: 'date', label: 'Date' }, { key: 'nightly', label: 'Nightly°C', cls: 'text-orange-400 font-medium' }, { key: 'baseline', label: 'Baseline' }, { key: 'deviation', label: 'Δ°C', cls: 'text-yellow-400' }]} />
              </CardContent>
            </Card>
          </TabsContent>

          {/* SLEEP */}
          <TabsContent value="sleep" className="mt-4">
            <Card>
              <CardHeader><CardTitle>Sleep Stage Breakdown</CardTitle></CardHeader>
              <CardContent>
                <div className="h-[280px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={data.sleep.slice(-30)} margin={{ left: -20, right: 8, top: 4 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                      <XAxis dataKey="date" {...ax} /><YAxis {...ax} />
                      <Tooltip contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }} />
                      <Bar dataKey="deep" stackId="a" fill="#312e81" name="Deep (hrs)" />
                      <Bar dataKey="rem" stackId="a" fill="#7c3aed" name="REM (hrs)" />
                      <Bar dataKey="light" stackId="a" fill="#c084fc" name="Light (hrs)" />
                      <Bar dataKey="awake" stackId="a" fill="#fbbf24" name="Awake (hrs)" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
                <DataTable rows={data.sleep} cols={[{ key: 'date', label: 'Date' }, { key: 'hours', label: 'Total', cls: 'text-foreground font-medium' }, { key: 'deep', label: 'Deep', cls: 'text-indigo-400' }, { key: 'rem', label: 'REM', cls: 'text-violet-400' }, { key: 'light', label: 'Light', cls: 'text-purple-300' }, { key: 'awake', label: 'Awake', cls: 'text-amber-400' }]} />
              </CardContent>
            </Card>
          </TabsContent>

          {/* RAW DATA EXPLORER */}
          <TabsContent value="raw" className="mt-4 space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-lg font-semibold">Raw Data Explorer</h2>
                <p className="text-sm text-muted-foreground">All {ENDPOINTS.length} available Google Health API endpoints (last 90 days)</p>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {ENDPOINTS.map(ep => (
                <Card key={ep.type} className="p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-base">{ep.icon}</span>
                        <span className="font-medium text-sm">{ep.label}</span>
                        <code className="text-xs bg-muted px-1.5 py-0.5 rounded text-muted-foreground">{ep.type}</code>
                      </div>
                      <p className="text-xs text-muted-foreground leading-relaxed">{ep.description}</p>
                      {rawCache[ep.type] && (
                        <div className="mt-2 text-xs text-emerald-400">
                          ✓ {(rawCache[ep.type].dataPoints || rawCache[ep.type].rollupDataPoints || []).length} records loaded
                        </div>
                      )}
                    </div>
                    <div className="flex flex-col gap-1.5 shrink-0">
                      <Button size="sm" variant="outline" className="h-7 text-xs px-2" onClick={() => fetchRaw(ep.type)} disabled={loadingRaw[ep.type]}>
                        {loadingRaw[ep.type] ? <Loader2 className="h-3 w-3 animate-spin" /> : 'Preview'}
                      </Button>
                      {rawCache[ep.type] && (
                        <Button size="sm" variant="outline" className="h-7 text-xs px-2 text-emerald-400 border-emerald-500/30" onClick={() => downloadJSON(ep.type)}>
                          <Download className="h-3 w-3 mr-1" />JSON
                        </Button>
                      )}
                    </div>
                  </div>
                  {rawCache[ep.type] && (
                    <pre className="mt-3 bg-muted rounded p-2 text-xs overflow-auto max-h-40 text-muted-foreground">
                      {JSON.stringify((rawCache[ep.type].dataPoints || rawCache[ep.type].rollupDataPoints || [])[0] || rawCache[ep.type], null, 2)}
                    </pre>
                  )}
                </Card>
              ))}
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
