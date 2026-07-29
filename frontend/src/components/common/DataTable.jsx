import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../ui/table.jsx';

/**
 * 轻量数据展示表格 —— 无排序、无搜索，纯数据渲染。
 * 用于各弹窗内部的数据明细展示。
 */
export default function DataTable({ rows, cols }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          {cols.map(c => (
            <TableHead key={c.label} className={c.right ? 'text-right text-[11px]' : 'text-[11px]'}>
              {c.label}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((r, i) => (
          <TableRow key={i}>
            {cols.map(c => (
              <TableCell key={c.label} className={c.right ? 'text-right tabular-nums text-xs' : 'text-xs'}>
                {c.render ? (typeof c.render === 'function' ? c.render(r[c.field], r) : c.render) : (c.mono ? <span className="font-mono">{r[c.field]}</span> : r[c.field])}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
