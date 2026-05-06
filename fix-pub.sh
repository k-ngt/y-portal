#!/bin/bash
# /usr/local/bin/fix-pub
echo "public_htmlの権限を修復します..."
setfacl -R -m u:y-p-u:rwx ~/public_html 2>/dev/null
echo "public_htmlの権限を修復しました."
