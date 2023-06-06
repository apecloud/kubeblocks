Name:kbcli    
Version:0.5.0
Release:1
Summary:Kbcli is command line tool for kubeblocks.    
Group:Applications/Internet
License:GPL
URL:kubeblocks.io    
Source0:kbcli-linux-arm64-v0.5.0.tar.gz
BuildRoot:    %(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)
#Requires:  
%global debug_package %{nil}  
%description
kbcli is command line tool for kubeblocks.
%prep
%setup -q -n linux-arm64
%build
%install
sudo chmod +x kbcli
mkdir -p %{buildroot}/usr/local/bin
sudo cp kbcli %{buildroot}/usr/local/bin
%clean
rm -rf %{buildroot}
%files
%defattr(-,root,root,-)
/usr/local/bin
%doc
#%changelog
