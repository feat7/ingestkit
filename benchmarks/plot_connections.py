#!/usr/bin/env python3
"""Regenerate two figures from independently audited, immutable schema 2 evidence."""
import argparse
from datetime import datetime
import gzip
import json
from pathlib import Path
import re
import textwrap

DEFAULT_STUDY=Path(__file__).resolve().parent/'results/connections-2026-11-04-7b54d30a'

def observed_points(run):
    observed=[];verified=[]
    for point in run['backlog']:
        value=point['Visible'] if point['ObservationAvailable'] else None
        target=verified if point['FinalVerification'] else observed
        target.append((point['elapsed_ns']/1e9,value))
    return observed,verified

def offset_lag(snapshot):
    if snapshot.get('Status')!='available':return None
    ends=snapshot.get('End',{});committed=snapshot.get('Committed',{})
    if set(ends)!=set(('0','1','2')) or set(committed)!=set(ends):return None
    if any(ends[k]<0 or committed[k]<-1 or committed[k]>ends[k] for k in ends):return None
    return sum(ends[k]-max(0,committed[k]) for k in ends)

def utc(value):
    value=re.sub(r'\.(\d+)',lambda m:'.'+m[1][:6].ljust(6,'0'),value)
    return datetime.fromisoformat(value.replace('Z','+00:00'))

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--study',type=Path,default=DEFAULT_STUDY)
    parser.add_argument('--out',type=Path,default=Path(__file__).resolve().parents[1]/'docs/assets')
    parser.add_argument('--preview',type=Path,help='optional PNG preview directory')
    args=parser.parse_args()
    import audit_study
    audit_study.audit(args.study,repo=Path(__file__).resolve().parents[1])
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    from matplotlib.ticker import FuncFormatter
    plt.rcParams.update({'font.family':'DejaVu Sans','font.size':10,'svg.hashsalt':'ingestkit-connections-v1','axes.spines.top':False,'axes.spines.right':False,'axes.edgecolor':'#c8d0d8','axes.labelcolor':'#273544','text.color':'#162636','xtick.color':'#415365','ytick.color':'#415365','grid.color':'#e4e9ee','axes.axisbelow':True})
    index=json.loads((args.study/'index.json').read_text())
    captures={row['Case']['ID']:audit_study.read_artifact(args.study,row['Artifact']) for row in index['Cases']}
    resources=audit_study.resource_samples(args.study,index)
    args.out.mkdir(parents=True,exist_ok=True)
    if args.preview:args.preview.mkdir(parents=True,exist_ok=True)
    def save(fig,name):
        fig.savefig(args.out/(name+'.svg'),facecolor='white',metadata={'Date':None,'Creator':'IngestKit plot_connections.py'})
        if args.preview:fig.savefig(args.preview/(name+'.png'),dpi=120,facecolor='white')
        plt.close(fig)
    kfmt=FuncFormatter(lambda x,_:f'{x/1000:g}k' if x else '0')
    for mobile in (False,True):
        suffix='-mobile' if mobile else ''
        fig,axes=plt.subplots(2,1,figsize=(3.8,8.2) if mobile else (8.6,5.8),gridspec_kw={'height_ratios':[1.3,1]},sharex=True)
        fig.subplots_adjust(left=.23 if mobile else .11,right=.96,bottom=.30 if mobile else .28,top=.77 if mobile else .78,hspace=.22)
        title='Four connections left\nthe client waiting' if mobile else 'More connections did not consistently improve acceptance'
        fig.suptitle(title,x=.07,y=.97,ha='left',fontsize=14,weight='bold')
        fig.text(.07,.855 if mobile else .88,'15,000 events/s offered · 128 workers\n1 KiB/event · 100 events/request\nThree 10s windows; idle retention 128' if mobile else '15,000 events/s offered · 128 workers\n1 KiB/event · 100 events/request · three 10s windows',fontsize=9 if mobile else 10,va='top')
        peaks=[]
        for i,cap in enumerate((4,8,16,32,64,128,0)):
            runs=captures[f'connections-cap-{cap}']['Report']['runs']
            peaks.append((min(r['connections']['PeakOpenTCP'] for r in runs),max(r['connections']['PeakOpenTCP'] for r in runs)))
            for rep,r in enumerate(runs):
                x=i+(rep-1)*.14;color=('#2475bd','#458a70','#b87929')[rep]
                axes[0].scatter(x,r['AcceptedEPS'],s=34,color=color,label=f'Repetition {rep+1}' if i==0 else None,zorder=3)
                mean=r['connections']['ConnectionAcquireNS']/max(1,len(r['request_samples']))/1e6
                axes[1].scatter(x,mean,s=34,color=color,zorder=3)
        axes[0].axhline(15000,color='#77838e',linestyle='--',linewidth=1)
        axes[0].set_ylim(0,16500);axes[0].set_ylabel('Acknowledged events/s');axes[0].yaxis.set_major_formatter(kfmt)
        axes[0].legend(frameon=False,fontsize=8,ncol=1 if mobile else 3,loc='lower right')
        axes[1].set_ylim(0,1050);axes[1].set_yticks([0,250,500,750,1000])
        axes[1].set_ylabel('Mean acquisition (ms)');axes[1].set_xlabel('Configured TCP cap\nObserved peak TCP',labelpad=10)
        labels=[str(c) if c else 'No cap' for c in (4,8,16,32,64,128,0)]
        for i,(a,b) in enumerate(peaks):labels[i]+='\n'+(str(a) if a==b else f'{a}–{b}')
        axes[1].set_xticks(range(7),labels,fontsize=8 if mobile else 10)
        for ax in axes:ax.grid(axis='y');ax.set_xlim(-.5,6.5)
        caption='Each dot is one repetition. Acquisition includes waiting, dialing and TLS. Idle retention stayed at 128. Limited outcomes are retained; these short local screens establish no sustained capacity.'
        fig.text(.07,.14 if mobile else .09,textwrap.fill('Each dot is a repetition. Acquisition includes waiting, dialing and TLS. These local 10s screens establish no sustained capacity.' if mobile else caption,43 if mobile else 112),fontsize=9,va='top',linespacing=1.45)
        save(fig,'connection-pressure'+suffix)
        # The predeclared 50k cohort, all repetitions. Final verification is a
        # separate marker; an unavailable observation or offset probe is a gap.
        runs=captures['connections-rate-50000']['Report']['runs']
        fig=plt.figure(figsize=(3.8,13.6) if mobile else (8.6,6.2))
        fig.subplots_adjust(left=.2 if mobile else .1,right=.96,top=.80 if mobile else .75,bottom=.15 if mobile else .22,hspace=.25 if mobile else .58,wspace=.15)
        grid=fig.add_gridspec(8 if mobile else 2,1 if mobile else 3,height_ratios=[2,1,.7,2,1,.7,2,1] if mobile else [2,1])
        fig.suptitle('The load ended before\nverification did' if mobile else 'The load ended before verification did',x=.07,y=.97,ha='left',fontsize=14,weight='bold')
        fig.text(.07,.918 if mobile else .89,'50,000 events/s offered\n500,000 planned per window\n1 KiB/event · 100 events/request' if mobile else '50,000 events/s offered · 500,000 planned/window\n1 KiB/event · 100 events/request · all three repetitions',fontsize=9,va='top')
        bottom_axes=[]
        for i,r in enumerate(runs):
            top=fig.add_subplot(grid[3*i if mobile else 0,0 if mobile else i]);bottom=fig.add_subplot(grid[3*i+1 if mobile else 1,0 if mobile else i],sharex=top)
            ack=[(0,0)];total=0
            for q in sorted(r['request_samples'],key=lambda q:q['FinishedNS']):
                if q['Accepted']:total+=q['Events'];ack.append((q['FinishedNS']/1e9,total))
            ack.append((r['TotalNS']/1e9,total))
            observed,verified=observed_points(r)
            top.step(*zip(*ack),where='post',color='#2475bd',label='202 acknowledged',linewidth=1.5)
            if observed:top.plot([x for x,_ in observed],[float('nan') if y is None else y for _,y in observed],color='#458a70',linestyle='--',label='Rows observed',linewidth=1.5)
            for x,y in verified:
                if y is not None:top.scatter(x,y,color='#162636',marker='D',s=25,label='Final ID/content check',zorder=4)
            bottom_axes.append(bottom)
            top.set_title(f'Repetition {i+1}',loc='left',fontsize=10,weight='bold');top.set_ylim(0,110000);top.yaxis.set_major_formatter(kfmt);top.tick_params(labelbottom=False)
            if mobile or i==0:top.set_ylabel('Cumulative events')
            else:top.tick_params(labelleft=False)
            if i==0:handles,labels=top.get_legend_handles_labels()
            start=utc(r['started_at']);points=[]
            for sample in resources:
                snap=sample.get('runtime',{}).get('offset_progress',{})
                at=snap.get('At')
                if at:
                    x=(utc(at)-start).total_seconds()
                    if 0<=x<=r['TotalNS']/1e9:points.append((x,offset_lag(snap)))
            bottom.plot([x for x,_ in points],[float('nan') if y is None else y for _,y in points],marker='.',color='#996631',linewidth=1)
            bottom.set_ylim(bottom=0);bottom.yaxis.set_major_formatter(kfmt);bottom.set_xlabel('' if mobile and i<2 else 'Seconds since window start')
            if mobile or i==0:bottom.set_ylabel('Queue-position lag')
            for ax in (top,bottom):ax.axvline(10,color='#77838e',linestyle=':',linewidth=1);ax.grid(axis='y');ax.set_xlim(0,max(x['TotalNS'] for x in runs)/1e9+2)
        upper=max(ax.get_ylim()[1] for ax in bottom_axes)
        for ax in bottom_axes:ax.set_ylim(0,upper)
        fig.legend(handles,labels,loc='upper left',bbox_to_anchor=(.07,.875 if mobile else .84),ncol=1 if mobile else 3,fontsize=8,frameon=False)
        caption='Dotted line: load window ends at 10s. Observed rows can include uncertain publishes. Offset snapshots are coarse and read separately; gaps mean unavailable probes. The final check includes a bounded wait for uncertain IDs, so its duration is not SQL write capacity.'
        fig.text(.07,.105 if mobile else .12,textwrap.fill(caption,43 if mobile else 116),fontsize=9,va='top',linespacing=1.4)
        save(fig,'acceptance-storage'+suffix)

if __name__=='__main__':main()
