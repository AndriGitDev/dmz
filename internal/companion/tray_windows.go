//go:build windows

package companion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// RunStatusIcon owns a Windows notification-area host. PowerShell and WinForms
// are OS components; no provider key is passed to this helper.
func RunStatusIcon(ctx context.Context, descriptor string) error {
	if validateLaunchPath("windows", descriptor) != nil {
		return errors.New("tray_unavailable")
	}
	dir, err := os.MkdirTemp("", "dmz-tray-")
	if err != nil {
		return errors.New("tray_unavailable")
	}
	defer os.RemoveAll(dir)
	executable, err := os.Executable()
	if err != nil || validateLaunchPath("windows", executable) != nil {
		return errors.New("tray_unavailable")
	}
	name := filepath.Join(dir, "status-icon.ps1")
	if os.WriteFile(name, []byte(windowsStatusIcon), 0600) != nil {
		return errors.New("tray_unavailable")
	}
	program := windowsSystemTool(hostPlatformEnv(), `WindowsPowerShell\v1.0\powershell.exe`)
	command := exec.CommandContext(ctx, program, "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-File", name, "-ControlFile", descriptor, "-Executable", executable)
	applyPlatformProcAttr(command, platformCommand{NoWindow: true})
	if command.Run() != nil && ctx.Err() == nil {
		return errors.New("tray_unavailable")
	}
	return nil
}

const windowsStatusIcon = `param([Parameter(Mandatory=$true)][string]$ControlFile,[Parameter(Mandatory=$true)][string]$Executable)
$ErrorActionPreference='Stop'
[System.Net.WebRequest]::DefaultWebProxy=$null
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$state=$null
for($i=0;$i -lt 100;$i++){
  try{$state=Get-Content -LiteralPath $ControlFile -Raw | ConvertFrom-Json;if($state.origin -and $state.token){break}}catch{}
  Start-Sleep -Milliseconds 100
}
if(!$state -or $state.origin -notmatch '^http://127\.0\.0\.1:[1-9][0-9]{0,4}$' -or $state.token -notmatch '^[a-f0-9]{64}$'){exit 1}
$headers=@{Authorization=('Bearer '+$state.token)}
function Call-Dmz([string]$Path,[string]$Method='GET',$Payload=$null){
  $options=@{Uri=($state.origin+'/api/'+$Path);Method=$Method;Headers=$headers;ContentType='application/json';TimeoutSec=3}
  if($null -ne $Payload){$options.Body=($Payload | ConvertTo-Json -Compress)}
  Invoke-RestMethod @options
}
$menu=New-Object System.Windows.Forms.ContextMenuStrip
$title=$menu.Items.Add('DMZ - text masking');$title.Enabled=$false
$scope=$menu.Items.Add('Basic text masking; tool arguments unchanged');$scope.Enabled=$false
$codex=$menu.Items.Add('Codex: checking');$codex.Enabled=$false
$claude=$menu.Items.Add('Claude Code: checking');$claude.Enabled=$false
[void]$menu.Items.Add('-')
$settings=$menu.Items.Add('Open DMZ settings')
$quit=$menu.Items.Add('Quit DMZ')
$icon=New-Object System.Windows.Forms.NotifyIcon
$appIcon=[System.Drawing.Icon]::ExtractAssociatedIcon($Executable)
if($null -eq $appIcon){$appIcon=[System.Drawing.SystemIcons]::Application.Clone()}
$icon.Icon=$appIcon
$icon.Text='DMZ - checking connections'
$icon.ContextMenuStrip=$menu
$icon.Visible=$true
$open={try{$window=Call-Dmz 'window' 'POST';if($window.address -match ('^'+[regex]::Escape($state.origin)+'/#open=[a-f0-9]{64}$')){Start-Process -FilePath $window.address}}catch{}}
$settings.add_Click($open)
$icon.add_DoubleClick($open)
$script:routes=@{}
$codex.add_Click({$r=$script:routes['codex'];if($r){try{Call-Dmz 'background' 'POST' @{client='codex';connected=(!$r.routed)}}catch{}}})
$claude.add_Click({$r=$script:routes['claude'];if($r){try{Call-Dmz 'background' 'POST' @{client='claude';connected=(!$r.routed)}}catch{}}})
$quit.add_Click({
  $answer=[System.Windows.Forms.MessageBox]::Show('DMZ restores its routes. Tools opened afterwards connect directly without DMZ masking. Open DMZ again after your next login.','Quit DMZ?',[System.Windows.Forms.MessageBoxButtons]::YesNo,[System.Windows.Forms.MessageBoxIcon]::Warning,[System.Windows.Forms.MessageBoxDefaultButton]::Button2)
  if($answer -ne [System.Windows.Forms.DialogResult]::Yes){return}
  try{Call-Dmz 'quit' 'POST'}catch{return};[System.Windows.Forms.Application]::ExitThread()
})
$timer=New-Object System.Windows.Forms.Timer
$timer.Interval=2000
$script:failures=0
$timer.add_Tick({
  try{
    $view=Call-Dmz 'state';$script:failures=0
    if($view.background_enabled){
      foreach($r in $view.background){
        $script:routes[$r.client]=$r
        $label=switch($r.state){'not_detected'{'not detected'}'restart_required'{'routing configured, not confirmed'}'traffic_observed'{'model traffic observed'}'needs_attention'{'needs attention'}'disconnected'{'disconnected'}default{'checking'}}
        $action=if($r.routed){'Disconnect'}else{'Connect'}
        $label=$label+'; '+[string]$r.new_redactions+' new values masked'
        if($r.client -eq 'codex'){$codex.Text='Codex: '+$label+' ('+$action+')';$codex.Enabled=($r.detected -or $r.routed)}
        if($r.client -eq 'claude'){$claude.Text='Claude Code: '+$label+' ('+$action+')';$claude.Enabled=($r.detected -or $r.routed)}
      }
    }else{
    foreach($id in @('codex','claude')){
      $client=@($view.clients | Where-Object {$_.id -eq $id})[0]
      $active=@($view.sessions | Where-Object {$_.client -eq $id -and $_.state -ne 'stopped'})
      $requests=($active | Measure-Object -Property requests -Sum).Sum
      $label=if(!$client.available){'not detected'}elseif($requests -gt 0){'model traffic observed'}elseif($active.Count -gt 0){'waiting for model traffic'}else{'detected; not connected'}
      if($id -eq 'codex'){$codex.Text='Codex: '+$label}else{$claude.Text='Claude Code: '+$label}
    }
    }
    $icon.Text='DMZ - right-click for connection status'
  }catch{
    $script:failures++;$icon.Text='DMZ - connection unavailable'
    if($script:failures -ge 3){[System.Windows.Forms.Application]::ExitThread()}
  }
})
try{$timer.Start();[System.Windows.Forms.Application]::Run()}finally{$timer.Stop();$timer.Dispose();$icon.Visible=$false;$icon.Dispose();$appIcon.Dispose();$menu.Dispose()}
`
