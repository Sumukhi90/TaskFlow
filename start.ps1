$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Join-Path $projectRoot 'backend'
$frontendDir = Join-Path $projectRoot 'frontend'
$env:GOCACHE = Join-Path $projectRoot '.gocache'

function Test-LocalPort([int]$Port) {
	foreach ($address in @('127.0.0.1', '::1')) {
		$family = if ($address -eq '::1') { [System.Net.Sockets.AddressFamily]::InterNetworkV6 } else { [System.Net.Sockets.AddressFamily]::InterNetwork }
		$client = [System.Net.Sockets.TcpClient]::new($family)
		try {
			$connect = $client.BeginConnect($address, $Port, $null, $null)
			if ($connect.AsyncWaitHandle.WaitOne(300)) {
				$client.EndConnect($connect)
				return $true
			}
		} catch {
		} finally {
			$client.Dispose()
		}
	}
	return $false
}

function Test-HttpEndpoint([string]$Uri) {
    try {
        $response = Invoke-WebRequest -Uri $Uri -UseBasicParsing -TimeoutSec 1
        return $response.StatusCode -ge 200 -and $response.StatusCode -lt 400
    } catch {
        return $false
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required. Install Go, then run this script again.'
}
if (-not (Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
    throw 'Node.js and npm are required. Install them, then run this script again.'
}

if (-not (Test-Path (Join-Path $frontendDir 'node_modules\vite\bin\vite.js'))) {
    Write-Host 'Installing frontend dependencies...'
    Push-Location $frontendDir
    try { & npm.cmd install; if ($LASTEXITCODE -ne 0) { throw 'npm install failed.' } }
    finally { Pop-Location }
}

$logDir = Join-Path $projectRoot '.runlogs'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$started = @()

if (Test-LocalPort 8080) {
    Write-Host 'Backend already responds on port 8080.'
} else {
    $go = (Get-Command go).Source
    $started += Start-Process -FilePath $go -ArgumentList 'run .' -WorkingDirectory $backendDir -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput (Join-Path $logDir 'backend.log') -RedirectStandardError (Join-Path $logDir 'backend-error.log')
}

if (Test-LocalPort 5173) {
    Write-Host 'Frontend already responds on port 5173.'
} else {
	$node = (Get-Command node).Source
	$vite = Join-Path $frontendDir 'node_modules\vite\bin\vite.js'
	$started += Start-Process -FilePath $node -ArgumentList @($vite, '--strictPort') -WorkingDirectory $frontendDir -WindowStyle Hidden -PassThru `
		-RedirectStandardOutput (Join-Path $logDir 'frontend.log') -RedirectStandardError (Join-Path $logDir 'frontend-error.log')
}

for ($attempt = 0; $attempt -lt 30; $attempt++) {
    if ((Test-HttpEndpoint 'http://localhost:8080/health') -and (Test-HttpEndpoint 'http://localhost:5173')) { break }
    Start-Sleep -Seconds 1
}

if (-not (Test-HttpEndpoint 'http://localhost:8080/health') -or -not (Test-HttpEndpoint 'http://localhost:5173')) {
    throw "TaskFlow did not start both services. Check logs in $logDir."
}

Write-Host 'TaskFlow is ready: http://localhost:5173 (API: http://localhost:8080)'
if ($started.Count -gt 0) {
    Write-Host "Started process IDs: $($started.Id -join ', '). Stop them with Stop-Process -Id <PID>."
}
