# kbcli filename
$CLI_FILENAME = "kbcli"

$CLI_TMP_ROOT = ""
$ARTIFACT_TMP_FILE = ""

$REPO = "apecloud/kbcli"
$GITHUB = "https://api.github.com"
$GITLAB_REPO = "85948"
$GITLAB = "https://jihulab.com/api/v4/projects"
$COUNTRY_CODE = ""

Import-Module Microsoft.PowerShell.Utility

function getCountryCode() {
    return (Invoke-WebRequest -Uri "https://ifconfig.io/country_code" -UseBasicParsing | Select-Object -ExpandProperty Content).Trim()
}

# verify whether x86 or AMD64 ; not supported x86
function verifySupported {
    $arch = (Get-Process -Id $PID).StartInfo.EnvironmentVariables["PROCESSOR_ARCHITECTURE"]
    if ($arch -eq "AMD64") {
        Write-Output "AMD64 is supported..."
        return
    }
    Write-Output "No support for x86 systems"
    exit 1
}

function checkExistingCLI {
    if (Get-Command "kbcli" -ErrorAction SilentlyContinue) {
        # kbcli has installed
        $exist_cli = (Get-Command "kbcli" -CommandType Application).Source
        Write-Output "kbcli is detected: $exist_cli. Please uninstall first."
        exit 1
    }
    else {
        Write-Output "Installing kbcli..."
    }
}

function getLatestRelease {
    $latest_release = ""

    if ($COUNTRY_CODE -eq "CN") {
        $releaseURL = "$GITLAB/$GITLAB_REPO/repository/tags/latest"
        $latest_release = (Invoke-WebRequest -Uri $releaseURL | Select-String -Pattern "message" | Select-Object -First 1).Line
        $latest_release = ($latest_release -split ",")[1].Trim()
        $latest_release = ($latest_release -split ":")[1].Trim()
        $latest_release = $latest_release.Trim('"') 
        
    }   
    else {
        $releaseURL = "$GITHUB/repos/$REPO/releases/latest"
        $response = Invoke-WebRequest -Uri $releaseURL -ContentType "application/json" | Select-String -Pattern "tag_name"
        $json = $response | ConvertFrom-Json
        $latest_release = $json.tag_name
           
    }
    return $latest_release
}

$webClient = New-Object System.Net.WebClient
$isDownLoaded = $False
$Data =
$timeout = New-TimeSpan -Seconds 60 
function downloadFile {
    param (
        $LATEST_RELEASE_TAG
    )
    $CLI_ARTIFACT="${CLI_FILENAME}-windows-amd64-${LATEST_RELEASE_TAG}.zip"
    $DOWNLOAD_BASE = "https://github.com/$REPO/releases/download"
    if ($COUNTRY_CODE -eq "CN") {
        $DOWNLOAD_BASE = "$GITLAB/$GITLAB_REPO/packages/generic/kubeblocks"
    }
    $DOWNLOAD_URL = "${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${CLI_ARTIFACT}"
    # Check the Resource 
    # Write-Host DOWNLOAD_URL = $DOWNLOAD_URL
    
    $webRequest = [System.Net.HttpWebRequest]::Create($DOWNLOAD_URL)
    $webRequest.Method = "HEAD"
    try {
        $webResponse = $webRequest.GetResponse()
        Write-Host "Resource has been found"
    } catch {
        Write-Host "Resource not found."
        exit 1
    }
    # Create the temp directory
    $CLI_TMP_ROOT = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "kbcli-install-$(Get-Date -Format 'yyyyMMddHHmmss')") 
    $Global:ARTIFACT_TMP_FILE = Join-Path $CLI_TMP_ROOT $CLI_ARTIFACT
    
    Register-ObjectEvent -InputObject $webClient -EventName DownloadFileCompleted `
        -Action {
        $Global:isDownLoaded = $True
        $timer.Stop()
        $webClient.Dispose()
    } -SourceIdentifier "DownloadFileCompleted"  | Out-Null
    
    Register-ObjectEvent -InputObject $webClient -EventName DownloadProgressChanged `
        -SourceIdentifier "DownloadProgressChanged" -Action {
        $Global:Data = $event
    } | Out-Null

    $Global:isDownLoaded = $False
    $timer = New-Object System.Timers.Timer
    $timer.Interval = 500
    
    Register-ObjectEvent -InputObject $timer -EventName Elapsed -SourceIdentifier "TimerElapsed" -Action {
        $precent = $Global:Data.SourceArgs.ProgressPercentage
        $totalBytes = $Global:Data.SourceArgs.TotalBytesToReceive
        $receivedBytes = $Global:Data.SourceArgs.BytesReceived
        if ($precent -ne $null) {
            $downloadProgress = [Math]::Round(($receivedBytes / $totalBytes) * 100, 2)
            $status = "Downloaded {0} of {1} bytes" -f $receivedBytes, $totalBytes
            Write-Progress -Activity "Downloading kbcli..." -Status $status -PercentComplete $downloadProgress
        }
    } | Out-Null

    try {
        $webClient.DownloadFileAsync($DOWNLOAD_URL, $Global:ARTIFACT_TMP_FILE)
        $timer.Start() 
    }
    catch {
        Write-Host "Download Failed"
        exit 1;
    }  
    
    while (-not $Global:isDownLoaded) {

    }
    Unregister-Event -SourceIdentifier "DownloadFileCompleted"
    Unregister-Event -SourceIdentifier "DownloadProgressChanged"
    Unregister-Event -SourceIdentifier "TimerElapsed"
    Write-Host "Download Completed"   
    return $CLI_TMP_ROOT 
}

function installFile {
    $DIR_NAME = "kbcli-windows-amd64"
    $kbcliexe = "kbcli.exe"
    $installPath = Join-Path "C:\Program Files"  $DIR_NAME
    
    if (!(Test-Path -Path $installPath -PathType Container)) {
        New-Item -ItemType Directory -Path $installPath | Out-Null
    }

    $tmp_root_kbcli = Join-Path $CLI_TMP_ROOT "windows-amd64" #Must match the folder name with workflow
    $tmp_root_kbcli = Join-Path $tmp_root_kbcli $kbcliexe  
    
    Expand-Archive -Path "$Global:ARTIFACT_TMP_FILE" -DestinationPath $CLI_TMP_ROOT
    
    if ($? -ne $True -or !(Test-Path $tmp_root_kbcli -PathType Leaf) ) {
        throw "Failed to unpack kbcli executable."
    }
    
    $envPath = [Environment]::GetEnvironmentVariable("Path", "User") # add to PATH
    if ($envPath -notlike "*$installPath*") {
        [Environment]::SetEnvironmentVariable("Path", "$envPath;$installPath", "User")
        Set-Item -Path Env:Path -Value $env:Path
    }

    Copy-Item -Path $tmp_root_kbcli -Destination $installPath
    if ( $? -eq $True -and (Test-Path (Join-Path $installPath $kbcliexe) -PathType Leaf) ) {
        Write-Host "kbcli installed successfully."
        Write-Host ""
        Write-Host "Make sure your docker service is running and begin your journey with kbcli:`n"
        Write-Host "`t$CLI_FILENAME playground init`n"
    } else {
        throw "Failed to install $CLI_FILENAME"
    }
}

function cleanup {
    if (Test-Path $CLI_TMP_ROOT) {
        Remove-Item $CLI_TMP_ROOT -Recurse -Force
    }
}

function installCompleted {
    Write-Host "`nFor more information on how to get started, please visit:"
    Write-Host "https://kubeblocks.io"
   
}
# ---------------------------------------
# main
# ---------------------------------------

verifySupported
checkExistingCLI
$COUNTRY_CODE = getCountryCode
$ret_val

if (-not $args) {
    Write-Host "Getting the latest kbcli ..."
    $ret_val = getLatestRelease
}
elseif ($args[0] -match "^v.*$") {
    $ret_val = $args[0]
}
else {
    $ret_val = "v" + $args[0]
}

$CLI_TMP_ROOT = downloadFile $ret_val
try {
    installFile  
} catch {
    Write-Host "An error occurred: $($_.Exception.Message)"
    Write-Host "Please try again in administrator mode!"
    cleanup
    exit 1
}  

cleanup 
installCompleted


