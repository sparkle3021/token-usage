import { useCallback, useState } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog.jsx';
import { Button } from '@/components/ui/button.jsx';
import { DatabaseIcon } from 'lucide-react';

export default function ImportDialog({ onRefresh }) {
  const [open, setOpen] = useState(false);
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState(null);

  const importDB = useCallback(async () => {
    setImporting(true);
    setResult(null);
    try {
      const r = await window.go.main.App.ImportCCSwitchDB();
      setResult(r);
      if (!r.error && onRefresh) onRefresh();
    } catch (err) {
      setResult({ error: String(err) });
    } finally {
      setImporting(false);
    }
  }, [onRefresh]);

  const close = useCallback(() => {
    setOpen(false);
    setResult(null);
  }, []);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button size="sm" variant="outline" onClick={() => setOpen(true)}>导入 CC-Switch 数据</Button>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>导入 CC-Switch 数据</DialogTitle>
        </DialogHeader>

        {result ? (
          <div className="space-y-3 py-4 text-center">
            {result.error ? (
              <p className="text-sm text-red-500">{result.error}</p>
            ) : (
              <div>
                <p className="text-sm text-green-600 font-medium">操作成功</p>
                <p className="text-xs text-muted-foreground mt-1">
                  {result.message || `共 ${result.total} 条记录，导入 ${result.imported} 条`}
                </p>
              </div>
            )}
            <Button size="sm" onClick={close} className="mt-2">关闭</Button>
          </div>
        ) : (
          <div className="space-y-3 py-4">
            <button
              onClick={importDB}
              disabled={importing}
              className="w-full flex items-center gap-3 p-3 rounded-lg border border-input hover:bg-accent transition-colors text-left disabled:opacity-50"
            >
              <DatabaseIcon className="size-5 text-muted-foreground shrink-0" />
              <div>
                <p className="text-sm font-medium">SQLite 数据库</p>
                <p className="text-xs text-muted-foreground">从 cc-switch 数据库读取增量数据</p>
              </div>
            </button>
            {importing && <p className="text-xs text-center text-muted-foreground pt-2">导入中…</p>}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
