# PrinterInstaller 核心测试用例（macOS + Windows）

> 仅覆盖核心验收：打印机出现、默认、删除、任务可见。
> 平台：`双` = 两平台都测；`mac`/`win` = 仅该平台。第一列勾选框用于执行时打勾。

---

## 1. 安装后打印机出现且默认

<table>

  <tr><th>完成</th><th>用例</th><th>平台</th><th>验证要点</th><th>预期结果</th></tr>

  <tr><td><input type="checkbox"></td><td>TC-01 正常安装</td><td>双</td><td>安装目标位置打印机后打开系统设置</td><td>打印机出现在设置列表，且标记为默认</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-02 多台打印机</td><td>双</td><td>安装含多台打印机的位置</td><td>全部出现在列表；仅第一台为默认</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-03 已存在+跳过</td><td>双</td><td>目标打印机已全部安装，选择「跳过」</td><td>提示「无需操作」，不授权，列表无变化</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-04 覆盖重装</td><td>双</td><td>目标打印机已存在，选择「覆盖」</td><td>重装后仍在列表且为默认</td></tr>

</table>

---

## 2. 删除后打印机消失

<table>

  <tr><th>完成</th><th>用例</th><th>平台</th><th>验证要点</th><th>预期结果</th></tr>

  <tr><td><input type="checkbox"></td><td>TC-05 手动勾选删除</td><td>双</td><td>勾选打印机执行删除后查看系统设置</td><td>勾选的打印机从列表消失</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-06 防误删</td><td>双</td><td>目标位置打印机已在本机，查看删除列表</td><td>目标打印机复选框禁用，不可勾选</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-07 覆盖清理</td><td>双</td><td>目标位置已有旧打印机，执行「覆盖」</td><td>旧队列移除后重装，最终只保留新装打印机</td></tr>

</table>

---

## 3. 任务可见性（发送任务后能在打印机看到）

<table>

  <tr><th>完成</th><th>用例</th><th>平台</th><th>验证要点</th><th>预期结果</th></tr>

  <tr><td><input type="checkbox"></td><td>TC-08 发送测试任务</td><td>双</td><td>安装后打印测试页或发送文档任务，打开队列</td><td>任务出现在队列，正常完成，无错误</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-09 刷工牌取件</td><td>双</td><td>发送任务后到打印机刷工牌</td><td>任务在打印机可见并能取件，连通/驱动/认证正常</td></tr>

  <tr><td><input type="checkbox"></td><td>TC-10 删除后发任务</td><td>双</td><td>删除该打印机后尝试发送任务</td><td>无法发送到已删除打印机，系统无残留队列</td></tr>

</table>

---

## 执行顺序建议

1. TC-01 / TC-02 → 确认「出现 + 默认」
2. TC-08 / TC-09 → 确认「任务可见」
3. TC-04 / TC-07 → 确认「覆盖重装干净」
4. TC-05 / TC-06 → 确认「删除消失 + 防误删」
5. TC-03 / TC-10 → 边界（跳过 / 删除后）
