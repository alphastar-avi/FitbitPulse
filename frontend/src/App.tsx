import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Activity, Heart, Moon, Loader2, Download, TrendingUp, Zap } from "lucide-react"
import { Bar, BarChart, CartesianGrid, XAxis, Tooltip, ResponsiveContainer, LineChart, Line, YAxis } from "recharts"

const API_BASE = "http://localhost:8080/api/raw"

function App() {
  const [stepsData, setStepsData] = useState<any[]>([])
  const [heartRateData, setHeartRateData] = useState<any[]>([])
  const [sleepData, setSleepData] = useState<any[]>([])
  const [summary, setSummary] = useState({ avgSteps: 0, avgHr: 0, avgSleep: 0 })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchData() {
      try {
        const [stepsRes, hrRes, sleepRes] = await Promise.all([
          fetch(`${API_BASE}?type=steps`),
          fetch(`${API_BASE}?type=daily-resting-heart-rate`),
          fetch(`${API_BASE}?type=sleep`)
        ])
        
        const stepsJson = await stepsRes.json()
        const hrJson = await hrRes.json()
        const sleepJson = await sleepRes.json()

        let pSteps: any[] = []
        let pHr: any[] = []
        let pSleep: any[] = []

        // Process Steps (Daily Rollup)
        if (stepsJson.rollupDataPoints) {
          pSteps = stepsJson.rollupDataPoints
            .filter((d: any) => d.steps && d.steps.countSum)
            .map((d: any) => {
              const date = new Date(d.civilStartTime.date.year, d.civilStartTime.date.month - 1, d.civilStartTime.date.day)
              return {
                rawDate: date,
                date: date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
                steps: parseInt(d.steps.countSum)
              }
            })
            .sort((a: any, b: any) => a.rawDate - b.rawDate)
          setStepsData(pSteps)
        }

        // Process Heart Rate
        if (hrJson.dataPoints) {
          pHr = hrJson.dataPoints.map((d: any) => {
            const date = new Date(d.dailyRestingHeartRate.date.year, d.dailyRestingHeartRate.date.month - 1, d.dailyRestingHeartRate.date.day)
            return {
              rawDate: date,
              date: date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
              bpm: parseInt(d.dailyRestingHeartRate.beatsPerMinute)
            }
          }).sort((a: any, b: any) => a.rawDate - b.rawDate)
          setHeartRateData(pHr)
        }

        // Process Sleep
        if (sleepJson.dataPoints) {
          pSleep = sleepJson.dataPoints.map((d: any) => {
            const start = new Date(d.sleep.interval.startTime)
            return {
              rawDate: start,
              date: start.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
              hours: parseFloat((parseInt(d.sleep.summary.minutesAsleep) / 60).toFixed(1)),
              deep: parseFloat((parseInt(d.sleep.summary.stagesSummary.find((s:any) => s.type === 'DEEP')?.minutes || 0) / 60).toFixed(1)),
              rem: parseFloat((parseInt(d.sleep.summary.stagesSummary.find((s:any) => s.type === 'REM')?.minutes || 0) / 60).toFixed(1)),
              light: parseFloat((parseInt(d.sleep.summary.stagesSummary.find((s:any) => s.type === 'LIGHT')?.minutes || 0) / 60).toFixed(1)),
            }
          }).sort((a: any, b: any) => a.rawDate - b.rawDate)
          setSleepData(pSleep)
        }

        // Calculate Summaries
        const avgSteps = pSteps.length > 0 ? pSteps.reduce((acc, curr) => acc + curr.steps, 0) / pSteps.length : 0
        const avgHr = pHr.length > 0 ? pHr.reduce((acc, curr) => acc + curr.bpm, 0) / pHr.length : 0
        const avgSleep = pSleep.length > 0 ? pSleep.reduce((acc, curr) => acc + curr.hours, 0) / pSleep.length : 0

        setSummary({
          avgSteps: Math.round(avgSteps),
          avgHr: Math.round(avgHr),
          avgSleep: parseFloat(avgSleep.toFixed(1))
        })

      } catch (e) {
        console.error("Failed to fetch data:", e)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  const downloadCSV = () => {
    // Combine data by date
    const dateMap = new Map()
    
    stepsData.forEach(d => {
      dateMap.set(d.date, { ...dateMap.get(d.date), Date: d.date, Steps: d.steps })
    })
    heartRateData.forEach(d => {
      dateMap.set(d.date, { ...dateMap.get(d.date), Date: d.date, 'Resting HR (BPM)': d.bpm })
    })
    sleepData.forEach(d => {
      dateMap.set(d.date, { ...dateMap.get(d.date), Date: d.date, 'Sleep (Hours)': d.hours, 'Deep Sleep (Hours)': d.deep, 'REM Sleep (Hours)': d.rem })
    })

    const combined = Array.from(dateMap.values())
    if (combined.length === 0) return

    const headers = ['Date', 'Steps', 'Resting HR (BPM)', 'Sleep (Hours)', 'Deep Sleep (Hours)', 'REM Sleep (Hours)']
    
    const csvContent = [
      headers.join(','),
      ...combined.map(row => 
        headers.map(header => row[header] !== undefined ? row[header] : '').join(',')
      )
    ].join('\n')

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    const url = URL.createObjectURL(blob)
    link.setAttribute('href', url)
    link.setAttribute('download', `fitbit_pulse_export_${new Date().toISOString().split('T')[0]}.csv`)
    link.style.visibility = 'hidden'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  if (loading) {
    return (
      <div className="flex h-screen w-full items-center justify-center bg-background text-foreground">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background text-foreground p-8 font-sans">
      <div className="max-w-5xl mx-auto space-y-8">
        <header className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <h1 className="text-4xl font-bold tracking-tight mb-2">FitbitPulse <span className="text-primary">Dashboard</span></h1>
            <p className="text-muted-foreground">Your detailed consolidated health metrics over the last 7 days.</p>
          </div>
          <Button onClick={downloadCSV} variant="outline" className="gap-2">
            <Download className="h-4 w-4" /> Export CSV
          </Button>
        </header>

        {/* Top Summary Cards */}
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Avg Daily Steps</CardTitle>
              <Activity className="h-4 w-4 text-emerald-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{summary.avgSteps.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground mt-1">Steps per day</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Avg Resting HR</CardTitle>
              <Heart className="h-4 w-4 text-rose-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{summary.avgHr}</div>
              <p className="text-xs text-muted-foreground mt-1">Beats per minute</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Avg Sleep Duration</CardTitle>
              <Moon className="h-4 w-4 text-purple-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{summary.avgSleep}h</div>
              <p className="text-xs text-muted-foreground mt-1">Hours per night</p>
            </CardContent>
          </Card>
        </div>

        <Tabs defaultValue="overview" className="space-y-4">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="sleep">Deep Sleep Details</TabsTrigger>
          </TabsList>
          
          <TabsContent value="overview" className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              
              {/* Daily Steps Card */}
              <Card className="col-span-1 lg:col-span-1">
                <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                  <CardTitle className="text-sm font-medium">Daily Steps</CardTitle>
                  <Activity className="h-4 w-4 text-emerald-500" />
                </CardHeader>
                <CardContent>
                  <div className="h-[250px] mt-4">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={stepsData} margin={{ top: 5, right: 0, left: -20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                        <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <YAxis stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <Tooltip 
                          contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }}
                          cursor={{fill: 'hsl(var(--muted))'}}
                        />
                        <Bar dataKey="steps" fill="#10b981" radius={[4, 4, 0, 0]} name="Steps" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </CardContent>
              </Card>

              {/* Resting Heart Rate Card */}
              <Card className="col-span-1 lg:col-span-1">
                <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                  <CardTitle className="text-sm font-medium">Resting Heart Rate</CardTitle>
                  <Heart className="h-4 w-4 text-rose-500" />
                </CardHeader>
                <CardContent>
                  <div className="h-[250px] mt-4">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={heartRateData} margin={{ top: 5, right: 0, left: -20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                        <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <YAxis domain={['dataMin - 5', 'dataMax + 5']} stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <Tooltip 
                          contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }}
                        />
                        <Line type="monotone" dataKey="bpm" name="BPM" stroke="#f43f5e" strokeWidth={3} dot={{ r: 4, fill: '#f43f5e', strokeWidth: 0 }} activeDot={{ r: 6 }} />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </CardContent>
              </Card>

              {/* Sleep Summary Card */}
              <Card className="col-span-1 lg:col-span-1">
                <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                  <CardTitle className="text-sm font-medium">Sleep Duration</CardTitle>
                  <Moon className="h-4 w-4 text-purple-500" />
                </CardHeader>
                <CardContent>
                  <div className="h-[250px] mt-4">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={sleepData} margin={{ top: 5, right: 0, left: -20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                        <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <YAxis stroke="hsl(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
                        <Tooltip 
                          contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }}
                          cursor={{fill: 'hsl(var(--muted))'}}
                        />
                        <Bar dataKey="hours" fill="#a855f7" radius={[4, 4, 0, 0]} name="Hours Asleep" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </CardContent>
              </Card>
              
            </div>
          </TabsContent>

          <TabsContent value="sleep" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Sleep Stages Breakdown</CardTitle>
                <CardDescription>Detailed view of Light, Deep, and REM sleep hours over time.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="h-[400px] w-full mt-4">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={sleepData} margin={{ top: 5, right: 20, left: -20, bottom: 5 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                      <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={12} tickLine={false} axisLine={false} />
                      <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} tickLine={false} axisLine={false} />
                      <Tooltip 
                        contentStyle={{ backgroundColor: 'hsl(var(--popover))', borderColor: 'hsl(var(--border))', color: 'hsl(var(--popover-foreground))', borderRadius: '8px' }}
                        cursor={{fill: 'hsl(var(--muted))'}}
                      />
                      <Bar dataKey="deep" stackId="a" fill="#4c1d95" name="Deep Sleep (hrs)" />
                      <Bar dataKey="rem" stackId="a" fill="#7e22ce" name="REM Sleep (hrs)" />
                      <Bar dataKey="light" stackId="a" fill="#c084fc" name="Light Sleep (hrs)" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}

export default App
