export NAME=my-scheduler-01
export NAMESPACE=my-scheduler
export SCHEDULERNAME=my-scheduler
export CONTEXT=${CONTEXT:=--context=idc-00}

eval "cat << EOF
$(< deploy.yaml)
EOF
" | kubectl $CONTEXT $1 -f -