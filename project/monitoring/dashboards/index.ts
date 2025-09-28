import * as fs from 'fs'
import * as path from 'path'

function loadDashboard(filename: string): string {
    const dashboardPath = path.join(__dirname, filename)
    return fs.readFileSync(dashboardPath, 'utf8')
}

export const dashboards = {
    'node-metrics.json': loadDashboard('node-metrics.json'),
    'coredns-metrics.json': loadDashboard('coredns-metrics.json'),
}
