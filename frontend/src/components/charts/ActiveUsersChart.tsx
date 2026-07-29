import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

interface DataPoint {
  date: string
  users: number
}

const tooltipStyle = {
  backgroundColor: '#0f172a',
  border: '1px solid #1e293b',
  borderRadius: '8px',
  color: '#f8fafc',
}

function fmtDate(val: string) {
  const d = new Date(val + 'T00:00:00')
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export function ActiveUsersChart({ data }: { data: DataPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <BarChart data={data} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
        <XAxis
          dataKey="date"
          tick={{ fill: '#64748b', fontSize: 11 }}
          tickLine={false}
          axisLine={false}
          tickFormatter={fmtDate}
        />
        <YAxis tick={{ fill: '#64748b', fontSize: 11 }} tickLine={false} axisLine={false} />
        <Tooltip cursor={{ fill: 'rgba(255,255,255,0.05)' }}
          contentStyle={tooltipStyle}
          formatter={(value) => [value ?? 0, 'Active Users']}
          labelStyle={{ color: '#f8fafc', marginBottom: '4px' }}
          itemStyle={{ color: '#c4b5fd' }}
        />
        <Bar dataKey="users" fill="#7c3aed" radius={[4, 4, 0, 0]} name="Active Users" />
      </BarChart>
    </ResponsiveContainer>
  )
}
