import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Activity, Heart, Moon, Loader2, Download, Wind, Thermometer, Brain } from "lucide-react"
import { Bar, BarChart, CartesianGrid, XAxis, Tooltip, ResponsiveContainer, LineChart, Line, YAxis } from "recharts"

const API_BASE = "/api/raw"

const tooltipStyle = {
  contentStyle: { backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' },
  cursor: { fill: 'hsl(var(--muted))' }
}

const axisProps = {
  stroke: "hsl(var(--muted-foreground))",
  fontSize: 11,
  tickLine: false,
  axisLine: false,
}

function MiniChart({ data, dataKey, color, type = 'bar' }: any) {
  return (
    <div className="h-[120px] mt-2">
      <ResponsiveContainer width="100%" height="100%">
        {type === 'line' ? (
          <LineChart data={data} margin={{ top: 4, right: 4, left: -30, bottom: 0 }}>
            <XAxis dataKey="date" {...axisProps} tick={false} />
            <YAxis {...axisProps} domain={['dataMin - 2', 'dataMax + 2']} />
            <Tooltip {...tooltipStyle} />
            <Line type="monotone" dataKey={dataKey} stroke={color} strokeWidth={2} dot={false} />
          </LineChart>
        ) : (
          <BarChart data={data} margin={{ top: 4, right: 4, left: -30, bottom: 0 }}>
            <XAxis dataKey="date" {...axisProps} tick={false} />
            <YAxis {...axisProps} />
            <Tooltip {...tooltipStyle} />
            <Bar dataKey={dataKey} fill={color} radius={[3, 3, 0, 0]} />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}

export default function App() {
  const [data, setData] = useState<any>({
    steps: [], hr: [], sleep: [], hrv: [], resp: [], wristTemp: []
  })

  const [summary, setSummary] = useState({ avgSteps: 0, avgHr: 0, avgSleep: 0, avgHrv: 0, avgResp: 0, avgTemp: 0 })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchAll() {
      try {
        const types = [
          'steps', 'daily-resting-heart-rate', 'sleep',
          'daily-heart-rate-variability', 'daily-respiratory-rate',
          'daily-sleep-temperature-derivations'
        ]
        const results = await Promise.all(
          types.map(t =>
            fetch(`${API_BASE}?type=${t}`)
              .then(r => { if (!r.ok) console.error(`Failed ${t}: ${r.status}`); return r.json() })
              .catch(e => { console.error(`Fetch error ${t}:`, e); return {} })
          )
        )
        const [stepsJ, hrJ, sleepJ, hrvJ, respJ, tempJ] = results
        console.log('Steps:', JSON.stringify(stepsJ).substring(0, 100))
        console.log('HR:', JSON.stringify(hrJ).substring(0, 100))
        console.log('Sleep:', JSON.stringify(sleepJ).substring(0, 100))
        console.log('HRV:', JSON.stringify(hrvJ).substring(0, 100))
        console.log('Resp:', JSON.stringify(respJ).substring(0, 100))
        console.log('Temp:', JSON.stringify(tempJ).substring(0, 100))



        // Steps
        const steps = (stepsJ.rollupDataPoints || [])
          .filter((d: any) => d.steps?.countSum)
          .map((d: any) => {
            const dt = new Date(d.civilStartTime.date.year, d.civilStartTime.date.month - 1, d.civilStartTime.date.day)
            return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }), steps: parseInt(d.steps.countSum) }
          }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        // HR
        const hr = (hrJ.dataPoints || []).map((d: any) => {
          const dt = new Date(d.dailyRestingHeartRate.date.year, d.dailyRestingHeartRate.date.month - 1, d.dailyRestingHeartRate.date.day)
          return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }), bpm: parseInt(d.dailyRestingHeartRate.beatsPerMinute) }
        }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        // Sleep
        const sleep = (sleepJ.dataPoints || []).filter((d: any) => d.sleep?.summary).map((d: any) => {
          const start = new Date(d.sleep.interval.startTime)
          const stages: any[] = d.sleep.summary.stagesSummary || []
          const findMins = (type: string) => parseFloat(((parseInt(stages.find((s: any) => s.type === type)?.minutes ?? '0') || 0) / 60).toFixed(1))
          return {
            rawDate: start,
            date: start.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
            hours: parseFloat(((parseInt(d.sleep.summary.minutesAsleep ?? '0') || 0) / 60).toFixed(1)),
            deep: findMins('DEEP'), rem: findMins('REM'), light: findMins('LIGHT'), awake: findMins('AWAKE')
          }
        }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        // HRV
        const hrv = (hrvJ.dataPoints || []).map((d: any) => {
          const h = d.dailyHeartRateVariability
          const dt = new Date(h.date.year, h.date.month - 1, h.date.day)
          return {
            rawDate: dt,
            date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
            hrv: h.averageHeartRateVariabilityMilliseconds ?? 0,
            entropy: parseFloat(Number(h.entropy || 0).toFixed(2)),
            rmssd: parseFloat(Number(h.deepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds || 0).toFixed(1))
          }
        }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        // Respiratory Rate
        const resp = (respJ.dataPoints || []).map((d: any) => {
          const r = d.dailyRespiratoryRate
          const dt = new Date(r.date.year, r.date.month - 1, r.date.day)
          return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }), bpm: r.breathsPerMinute }
        }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        // Wrist Temp
        const wristTemp = (tempJ.dataPoints || []).map((d: any) => {
          const t = d.dailySleepTemperatureDerivations
          const dt = new Date(t.date.year, t.date.month - 1, t.date.day)
          const nightly = parseFloat(Number(t.nightlyTemperatureCelsius || 0).toFixed(2))
          const baseline = parseFloat(Number(t.baselineTemperatureCelsius || 0).toFixed(2))
          const deviation = parseFloat((nightly - baseline).toFixed(2))
          return { rawDate: dt, date: dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }), nightly, baseline, deviation }
        }).sort((a: any, b: any) => a.rawDate - b.rawDate)

        setData({ steps, hr, sleep, hrv, resp, wristTemp })

        const avg = (arr: any[], key: string) => arr.length ? Math.round(arr.reduce((s, d) => s + d[key], 0) / arr.length * 10) / 10 : 0
        setSummary({
          avgSteps: avg(steps, 'steps'),
          avgHr: avg(hr, 'bpm'),
          avgSleep: avg(sleep, 'hours'),
          avgHrv: avg(hrv, 'hrv'),
          avgResp: avg(resp, 'bpm'),
          avgTemp: avg(wristTemp, 'nightly')
        })
      } catch (e) { console.error('Dashboard fetch error:', e) }
      finally { setLoading(false) }
    }
    fetchAll()
  }, [])

  const downloadCSV = () => {
    const rows: any[] = []

    // Sleep — every stage in detail
    data.sleep.forEach((d: any) => {
      rows.push({ Date: d.date, Metric: 'Sleep', TotalHours: d.hours, DeepHrs: d.deep, REMHrs: d.rem, LightHrs: d.light, AwakeHrs: d.awake })
    })
    data.steps.forEach((d: any) => {
      const idx = rows.findIndex(r => r.Date === d.date)
      if (idx >= 0) rows[idx].Steps = d.steps
      else rows.push({ Date: d.date, Metric: 'Activity', Steps: d.steps })
    })
    data.hr.forEach((d: any) => {
      const idx = rows.findIndex(r => r.Date === d.date)
      if (idx >= 0) rows[idx]['RestingHR_BPM'] = d.bpm
      else rows.push({ Date: d.date, RestingHR_BPM: d.bpm })
    })
    data.hrv.forEach((d: any) => {
      const idx = rows.findIndex(r => r.Date === d.date)
      if (idx >= 0) { rows[idx]['HRV_ms'] = d.hrv; rows[idx]['HRV_Entropy'] = d.entropy; rows[idx]['HRV_RMSSD_ms'] = d.rmssd }
      else rows.push({ Date: d.date, HRV_ms: d.hrv, HRV_Entropy: d.entropy, HRV_RMSSD_ms: d.rmssd })
    })
    data.resp.forEach((d: any) => {
      const idx = rows.findIndex(r => r.Date === d.date)
      if (idx >= 0) rows[idx]['BreathingRate_BPM'] = d.bpm
      else rows.push({ Date: d.date, BreathingRate_BPM: d.bpm })
    })
    data.wristTemp.forEach((d: any) => {
      const idx = rows.findIndex(r => r.Date === d.date)
      if (idx >= 0) { rows[idx]['WristTemp_Nightly_C'] = d.nightly; rows[idx]['WristTemp_Baseline_C'] = d.baseline; rows[idx]['WristTemp_Deviation_C'] = d.deviation }
      else rows.push({ Date: d.date, WristTemp_Nightly_C: d.nightly, WristTemp_Baseline_C: d.baseline, WristTemp_Deviation_C: d.deviation })
    })

    // Also append raw JSON dump as a second sheet separated
    const allHeaders = Array.from(new Set(rows.flatMap(Object.keys)))
    const csv = [
      allHeaders.join(','),
      ...rows.map(r => allHeaders.map(h => r[h] !== undefined ? r[h] : '').join(','))
    ].join('\n')

    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `fitbit_charge6_export_${new Date().toISOString().split('T')[0]}.csv`
    link.click()
  }

  if (loading) return (
    <div className="flex h-screen w-full items-center justify-center bg-background">
      <Loader2 className="h-8 w-8 animate-spin text-emerald-500" />
    </div>
  )

  return (
    <div className="min-h-screen bg-background text-foreground p-6 font-sans">
      <div className="max-w-6xl mx-auto space-y-6">

        {/* Header */}
        <header className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-medium bg-emerald-500/20 text-emerald-400 px-2 py-0.5 rounded-full">FITBIT PLATFORM</span>
              <span className="text-xs text-muted-foreground">Charge 6 · Last 7 days</span>
            </div>
            <h1 className="text-3xl font-bold tracking-tight">Fitbit Charge 6 <span className="text-emerald-500">Health Dashboard</span></h1>
            <p className="text-sm text-muted-foreground mt-1">All metrics sourced from your Fitbit Charge 6 via Google Health API</p>
          </div>
          <Button onClick={downloadCSV} variant="outline" size="sm" className="gap-2 shrink-0">
            <Download className="h-4 w-4" /> Export Full CSV
          </Button>
        </header>

        {/* Summary Row */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
          {[
            { label: 'Avg Steps', value: summary.avgSteps.toLocaleString(), unit: 'steps/day', color: 'text-emerald-400' },
            { label: 'Resting HR', value: summary.avgHr, unit: 'bpm', color: 'text-rose-400' },
            { label: 'Sleep', value: `${summary.avgSleep}h`, unit: 'per night', color: 'text-violet-400' },
            { label: 'HRV', value: `${summary.avgHrv}ms`, unit: 'avg HRV', color: 'text-blue-400' },
            { label: 'Breathing', value: `${summary.avgResp}`, unit: 'breaths/min', color: 'text-cyan-400' },
            { label: 'Wrist Temp', value: `${summary.avgTemp}°C`, unit: 'nightly avg', color: 'text-orange-400' },
          ].map(s => (
            <Card key={s.label} className="p-3">
              <p className="text-xs text-muted-foreground">{s.label}</p>
              <p className={`text-xl font-bold mt-0.5 ${s.color}`}>{s.value}</p>
              <p className="text-xs text-muted-foreground">{s.unit}</p>
            </Card>
          ))}
        </div>

        {/* Tabs */}
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="recovery">Recovery Metrics</TabsTrigger>
            <TabsTrigger value="sleep">Sleep Detail</TabsTrigger>
          </TabsList>

          {/* OVERVIEW */}
          <TabsContent value="overview" className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Daily Steps</CardTitle>
                <Activity className="h-4 w-4 text-emerald-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.steps} dataKey="steps" color="#10b981" />
                <div className="mt-2 grid grid-cols-2 gap-x-4 text-xs text-muted-foreground">
                  {data.steps.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span><span className="text-foreground font-medium">{d.steps.toLocaleString()}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Resting Heart Rate</CardTitle>
                <Heart className="h-4 w-4 text-rose-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.hr} dataKey="bpm" color="#f43f5e" type="line" />
                <div className="mt-2 grid grid-cols-2 gap-x-4 text-xs text-muted-foreground">
                  {data.hr.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span><span className="text-foreground font-medium">{d.bpm} bpm</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Sleep Duration</CardTitle>
                <Moon className="h-4 w-4 text-violet-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.sleep} dataKey="hours" color="#a855f7" />
                <div className="mt-2 grid grid-cols-2 gap-x-4 text-xs text-muted-foreground">
                  {data.sleep.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span><span className="text-foreground font-medium">{d.hours}h</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* RECOVERY */}
          <TabsContent value="recovery" className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">HRV (Heart Rate Variability)</CardTitle>
                <Brain className="h-4 w-4 text-blue-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.hrv} dataKey="hrv" color="#3b82f6" type="line" />
                <div className="mt-2 space-y-0.5 text-xs text-muted-foreground">
                  {data.hrv.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span>
                      <span className="text-foreground font-medium">{d.hrv}ms</span>
                      <span className="text-blue-400">RMSSD: {d.rmssd}ms</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Breathing Rate</CardTitle>
                <Wind className="h-4 w-4 text-cyan-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.resp} dataKey="bpm" color="#06b6d4" type="line" />
                <div className="mt-2 grid grid-cols-2 gap-x-4 text-xs text-muted-foreground">
                  {data.resp.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span><span className="text-foreground font-medium">{d.bpm} br/min</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-1 flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Wrist Skin Temperature</CardTitle>
                <Thermometer className="h-4 w-4 text-orange-500" />
              </CardHeader>
              <CardContent>
                <MiniChart data={data.wristTemp} dataKey="deviation" color="#f97316" type="line" />
                <div className="mt-2 text-xs text-muted-foreground space-y-0.5">
                  {data.wristTemp.map((d: any) => (
                    <div key={d.date} className="flex justify-between border-b border-border py-0.5">
                      <span>{d.date}</span>
                      <span className="text-foreground font-medium">{d.nightly}°C</span>
                      <span className={d.deviation >= 0 ? 'text-orange-400' : 'text-blue-400'}>{d.deviation > 0 ? '+' : ''}{d.deviation}°</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* SLEEP DETAIL */}
          <TabsContent value="sleep" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Sleep Stage Breakdown — per Night</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="h-[300px] w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={data.sleep} margin={{ left: -20, right: 8, top: 4 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                      <XAxis dataKey="date" {...axisProps} />
                      <YAxis {...axisProps} />
                      <Tooltip contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }} />
                      <Bar dataKey="deep" stackId="a" fill="#312e81" name="Deep (hrs)" />
                      <Bar dataKey="rem" stackId="a" fill="#7c3aed" name="REM (hrs)" />
                      <Bar dataKey="light" stackId="a" fill="#c084fc" name="Light (hrs)" />
                      <Bar dataKey="awake" stackId="a" fill="#fbbf24" name="Awake (hrs)" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
                <div className="mt-4 overflow-x-auto">
                  <table className="w-full text-xs text-muted-foreground border-collapse">
                    <thead>
                      <tr className="border-b border-border text-left">
                        {['Date', 'Total', 'Deep', 'REM', 'Light', 'Awake'].map(h => <th key={h} className="py-1 px-2 font-medium">{h}</th>)}
                      </tr>
                    </thead>
                    <tbody>
                      {data.sleep.map((d: any) => (
                        <tr key={d.date} className="border-b border-border hover:bg-muted/40">
                          <td className="py-1 px-2 text-foreground font-medium">{d.date}</td>
                          <td className="py-1 px-2 text-foreground">{d.hours}h</td>
                          <td className="py-1 px-2 text-indigo-400">{d.deep}h</td>
                          <td className="py-1 px-2 text-violet-400">{d.rem}h</td>
                          <td className="py-1 px-2 text-purple-300">{d.light}h</td>
                          <td className="py-1 px-2 text-amber-400">{d.awake}h</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
