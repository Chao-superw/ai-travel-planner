export interface TripTemplate {
  city: string
  region: string
  theme: string
  days: number
  budget_cents: number
  interests: string[]
  img: string
}

export const TEMPLATES: TripTemplate[] = [
  { city: '杭州市', region: '华东', theme: '西湖人文漫游', days: 3, budget_cents: 200000, interests: ['文化', '美食'], img: '/templates/hangzhou.jpg' },
  { city: '成都市', region: '西南', theme: '美食与熊猫', days: 4, budget_cents: 250000, interests: ['美食', '亲子'], img: '/templates/chengdu.jpg' },
  { city: '西安市', region: '西北', theme: '千年古都', days: 3, budget_cents: 220000, interests: ['文化', '摄影'], img: '/templates/xian.jpg' },
  { city: '厦门市', region: '华南', theme: '海岛慢生活', days: 4, budget_cents: 280000, interests: ['自然', '摄影'], img: '/templates/xiamen.jpg' },
  { city: '大理市', region: '西南', theme: '苍山洱海', days: 5, budget_cents: 300000, interests: ['自然', '摄影'], img: '/templates/dali.jpg' },
  { city: '北京市', region: '华北', theme: '皇城中轴线', days: 4, budget_cents: 260000, interests: ['文化', '亲子'], img: '/templates/beijing.jpg' },
]

export const QUICK_INTERESTS = ['全部', '文化', '美食', '自然', '亲子', '摄影'] as const
