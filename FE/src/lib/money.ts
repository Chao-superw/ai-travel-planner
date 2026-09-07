export function centsToYuan(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function budgetStatusText(complete: boolean, withinBudget?: boolean, unknownCount = 0): string {
  if (!complete) return `含 ${unknownCount} 项待确认费用`
  return withinBudget ? '未超预算' : '已超预算'
}
