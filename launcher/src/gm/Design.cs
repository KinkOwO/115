using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using System.Windows.Forms;

// Shared native controls: no browser runtime or game-client dependency.
class ActionButton : Button {
 public bool Accent,Danger;bool hover,pressed;
 public ActionButton(){DoubleBuffered=true;FlatStyle=FlatStyle.Flat;FlatAppearance.BorderSize=0;Cursor=Cursors.Hand;UseVisualStyleBackColor=false;}
 public override Size GetPreferredSize(Size proposed){var t=TextRenderer.MeasureText(Text,Font);return new Size(Math.Max(MinimumSize.Width,t.Width+Padding.Horizontal+8),Math.Max(MinimumSize.Height,Math.Max(38,t.Height+Padding.Vertical+16)));}
 protected override void OnMouseEnter(EventArgs e){hover=true;Invalidate();base.OnMouseEnter(e);}
 protected override void OnMouseLeave(EventArgs e){hover=pressed=false;Invalidate();base.OnMouseLeave(e);}
 protected override void OnMouseDown(MouseEventArgs e){pressed=true;Invalidate();base.OnMouseDown(e);}
 protected override void OnMouseUp(MouseEventArgs e){pressed=false;Invalidate();base.OnMouseUp(e);}
 protected override void OnGotFocus(EventArgs e){base.OnGotFocus(e);Invalidate();}
 protected override void OnLostFocus(EventArgs e){base.OnLostFocus(e);Invalidate();}
 protected override void OnPaint(PaintEventArgs e){var g=e.Graphics;g.SmoothingMode=SmoothingMode.AntiAlias;g.Clear(Parent==null?Color.FromArgb(23,28,35):Parent.BackColor);var fill=Accent?Color.FromArgb(232,190,111):Color.FromArgb(32,40,52);var stroke=Danger?Color.FromArgb(118,65,71):Color.FromArgb(58,68,84);var ink=Accent?Color.FromArgb(25,23,19):Danger?Color.FromArgb(247,155,160):Color.FromArgb(224,232,242);if(hover){fill=Accent?Color.FromArgb(248,208,137):Color.FromArgb(43,53,68);}if(pressed)fill=Accent?Color.FromArgb(207,162,85):Color.FromArgb(26,33,43);if(!Enabled){fill=Accent?Color.FromArgb(121,102,70):Color.FromArgb(29,36,46);ink=Accent?Color.FromArgb(225,214,193):Color.FromArgb(144,155,172);}using(var path=Surface.Shape(new Rectangle(0,0,Width-1,Height-1),6)){using(var b=new SolidBrush(fill))g.FillPath(b,path);if(!Accent)using(var p=new Pen(stroke))g.DrawPath(p,path);}TextRenderer.DrawText(g,Text,Font,ClientRectangle,ink,TextFormatFlags.HorizontalCenter|TextFormatFlags.VerticalCenter|TextFormatFlags.EndEllipsis);if(Focused&&ShowFocusCues)using(var p=new Pen(Color.FromArgb(232,190,111))){p.DashStyle=DashStyle.Dot;g.DrawRectangle(p,4,4,Width-9,Height-9);}}
}

class DarkComboBox : ComboBox {
 public DarkComboBox(){DrawMode=DrawMode.OwnerDrawFixed;ItemHeight=24;FlatStyle=FlatStyle.Flat;BackColor=Color.FromArgb(16,21,29);ForeColor=Color.FromArgb(234,239,247);}
 protected override void OnDrawItem(DrawItemEventArgs e){var selected=(e.State&DrawItemState.Selected)!=0;using(var b=new SolidBrush(selected?Color.FromArgb(58,49,35):BackColor))e.Graphics.FillRectangle(b,e.Bounds);string value=e.Index>=0&&e.Index<Items.Count?GetItemText(Items[e.Index]):Text;TextRenderer.DrawText(e.Graphics,value,Font,new Rectangle(e.Bounds.X+8,e.Bounds.Y,e.Bounds.Width-12,e.Bounds.Height),Enabled?ForeColor:Color.FromArgb(159,170,186),TextFormatFlags.VerticalCenter|TextFormatFlags.EndEllipsis);}
 void PaintClosed(Graphics g){g.SmoothingMode=SmoothingMode.AntiAlias;using(var fill=new SolidBrush(BackColor))g.FillRectangle(fill,ClientRectangle);TextRenderer.DrawText(g,Text,Font,new Rectangle(9,0,Width-35,Height),Enabled?ForeColor:Color.FromArgb(159,170,186),TextFormatFlags.VerticalCenter|TextFormatFlags.EndEllipsis);using(var pen=new Pen(Focused?Color.FromArgb(232,190,111):Color.FromArgb(62,74,91)))g.DrawRectangle(pen,0,0,Width-1,Height-1);using(var pen=new Pen(Color.FromArgb(165,178,197),1.5f)){g.DrawLine(pen,Width-16,Height/2-2,Width-12,Height/2+2);g.DrawLine(pen,Width-12,Height/2+2,Width-8,Height/2-2);}}
 protected override void WndProc(ref Message m){base.WndProc(ref m);if(m.Msg==0xF){using(var g=Graphics.FromHwnd(Handle))PaintClosed(g);}else if((m.Msg==0x317||m.Msg==0x318)&&m.WParam!=IntPtr.Zero){using(var g=Graphics.FromHdc(m.WParam))PaintClosed(g);}}
}

class DarkNumber : NumericUpDown {
 public DarkNumber(){NativeTheme.Dark(this);if(Controls.Count>0){var spin=Controls[0];NativeTheme.Dark(spin);spin.Paint+=(s,e)=>PaintSpin(e.Graphics,spin.ClientRectangle);}}
 void PaintSpin(Graphics g,Rectangle r){using(var b=new SolidBrush(Color.FromArgb(32,40,52)))g.FillRectangle(b,r);using(var p=new Pen(Enabled?Color.FromArgb(187,198,214):Color.FromArgb(113,126,146),1.25f)){float x=r.Width/2f,y=r.Height/4f;g.DrawLines(p,new[]{new PointF(x-3,y+1),new PointF(x,y-2),new PointF(x+3,y+1)});y=r.Height*3f/4f;g.DrawLines(p,new[]{new PointF(x-3,y-1),new PointF(x,y+2),new PointF(x+3,y-1)});}}
}

class EmptyItems : Control {
 public EmptyItems(){DoubleBuffered=true;ForeColor=Color.FromArgb(153,166,185);}
 protected override void OnPaint(PaintEventArgs e){base.OnPaint(e);int y=Height/2;var ink=ForeColor;if(Height>130){Glyphs.Draw(e.Graphics,new Rectangle(Width/2-15,y-58,30,30),1,Color.FromArgb(99,115,136));TextRenderer.DrawText(e.Graphics,"暂无物品",Font,new Rectangle(8,y-9,Width-16,25),ink,TextFormatFlags.HorizontalCenter);TextRenderer.DrawText(e.Graphics,"输入名称或编号搜索，或选择角色查看背包",Font,new Rectangle(8,y+20,Width-16,25),ink,TextFormatFlags.HorizontalCenter|TextFormatFlags.EndEllipsis);}else TextRenderer.DrawText(e.Graphics,"暂无物品 · 输入名称或编号搜索",Font,new Rectangle(8,0,Width-16,Height),ink,TextFormatFlags.HorizontalCenter|TextFormatFlags.VerticalCenter|TextFormatFlags.EndEllipsis);}
}

static class VisualIdentity {
 public static Image Mark(){var bitmap=new Bitmap(48,48);using(var g=Graphics.FromImage(bitmap)){g.SmoothingMode=SmoothingMode.AntiAlias;g.Clear(Color.FromArgb(23,28,35));using(var pen=new Pen(Color.FromArgb(232,190,111),1.5f))g.DrawPolygon(pen,new[]{new Point(24,4),new Point(42,14),new Point(42,34),new Point(24,44),new Point(6,34),new Point(6,14)});using(var font=new Font("Segoe UI",11F,FontStyle.Bold))TextRenderer.DrawText(g,"GM",font,new Rectangle(0,0,48,48),Color.FromArgb(232,190,111),TextFormatFlags.HorizontalCenter|TextFormatFlags.VerticalCenter);}return bitmap;}
}

static class NativeTheme {
 [DllImport("uxtheme.dll",CharSet=CharSet.Unicode)]static extern int SetWindowTheme(IntPtr handle,string subApp,string subId);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)]static extern IntPtr SendMessage(IntPtr handle,int msg,IntPtr wParam,string text);
 [DllImport("user32.dll")]public static extern bool ReleaseCapture();
 [DllImport("user32.dll")]public static extern IntPtr SendMessage(IntPtr handle,int msg,IntPtr wParam,IntPtr lParam);
 public static void Dark(Control control){Action apply=()=>{try{SetWindowTheme(control.Handle,"DarkMode_Explorer",null);}catch{}};if(control.IsHandleCreated)apply();else control.HandleCreated+=(s,e)=>apply();}
 public static void DarkTree(Control root){if(root is ScrollableControl||root is DataGridView)Dark(root);foreach(Control child in root.Controls)DarkTree(child);}
 public static void Cue(TextBox control,string text){control.HandleCreated+=(s,e)=>SendMessage(control.Handle,0x1501,new IntPtr(1),text);}
}

class SelectedItemView : Control {
 public string NameText="尚未选择物品",Detail="点击物品卡片查看并编辑";public ArtResult Art=new ArtResult();
 public SelectedItemView(){DoubleBuffered=true;Height=74;Dock=DockStyle.Top;}
 protected override void OnPaint(PaintEventArgs e){base.OnPaint(e);var g=e.Graphics;g.SmoothingMode=SmoothingMode.AntiAlias;var icon=new Rectangle(0,8,52,52);using(var b=new SolidBrush(Color.FromArgb(16,21,29)))using(var p=Surface.Shape(icon,8))g.FillPath(b,p);using(var p=new Pen(Color.FromArgb(64,69,82)))using(var shape=Surface.Shape(icon,8))g.DrawPath(p,shape);if(Art.Image!=null){g.InterpolationMode=InterpolationMode.NearestNeighbor;int w=Math.Min(44,Art.Image.Width),h=Math.Min(44,Art.Image.Height);g.DrawImage(Art.Image,new Rectangle(icon.X+(52-w)/2,icon.Y+(52-h)/2,w,h));}else Glyphs.Draw(g,new Rectangle(14,22,24,24),3,Color.FromArgb(145,156,170));using(var font=new Font(Font.FontFamily,11F,FontStyle.Bold))TextRenderer.DrawText(g,NameText,font,new Rectangle(66,11,Width-70,27),Art.Image==null?ForeColor:ClientArt.Rarity(Art.Rarity),TextFormatFlags.EndEllipsis);TextRenderer.DrawText(g,Detail,Font,new Rectangle(66,40,Width-70,24),Color.FromArgb(153,166,185),TextFormatFlags.EndEllipsis);}
}

partial class GMForm {
 static readonly Color Muted=Color.FromArgb(153,166,185);
 readonly Label pageHeading=new Label(),pageDescription=new Label();
 readonly SelectedItemView selectedItem=new SelectedItemView();
 int selectedArtGeneration;
 protected override void WndProc(ref Message m){base.WndProc(ref m);if(!TopLevel||m.Msg!=0x84||WindowState!=FormWindowState.Normal)return;long value=m.LParam.ToInt64();var p=PointToClient(new Point(unchecked((short)(value&0xffff)),unchecked((short)((value>>16)&0xffff))));bool left=p.X<6,right=p.X>=Width-6,top=p.Y<6,bottom=p.Y>=Height-6;int hit=top?(left?13:right?14:12):bottom?(left?16:right?17:15):left?10:right?11:0;if(hit!=0)m.Result=new IntPtr(hit);}
 Control WindowChrome(){var chrome=new Panel{Dock=DockStyle.Fill,Margin=new Padding(0),BackColor=bg};var title=new Label{Text="115US   ·   "+(Program.Cloud?"联网服主 GM":"单机 GM")+"     /     1.4.5",Dock=DockStyle.Fill,ForeColor=Muted,Font=new Font("Segoe UI",8F),TextAlign=ContentAlignment.MiddleLeft};Action move=()=>{if(WindowState==FormWindowState.Normal){NativeTheme.ReleaseCapture();NativeTheme.SendMessage(Handle,0xA1,new IntPtr(2),IntPtr.Zero);}};title.MouseDown+=(s,e)=>{if(e.Button==MouseButtons.Left)move();};title.DoubleClick+=(s,e)=>WindowState=WindowState==FormWindowState.Maximized?FormWindowState.Normal:FormWindowState.Maximized;chrome.Controls.Add(title);var controls=new FlowLayoutPanel{Dock=DockStyle.Right,Width=126,WrapContents=false,Margin=new Padding(0)};string[] captions={"−","□","×"};for(int i=0;i<3;i++){int action=i;var button=new ActionButton{Text=captions[i],Width=38,Height=26,Margin=new Padding(3,1,0,0),Danger=i==2,Font=new Font("Segoe UI",12F),TabStop=false};button.Click+=(s,e)=>{if(action==0)WindowState=FormWindowState.Minimized;else if(action==1)WindowState=WindowState==FormWindowState.Maximized?FormWindowState.Normal:FormWindowState.Maximized;else Close();};controls.Controls.Add(button);}chrome.Controls.Add(controls);return chrome;}
 static readonly string[] PageHints={"调整角色成长、货币与技能点","查看所有容器，编辑数量与装备打造","按部位、等级与品质查找装备","查找消耗品、材料和其他物品","查找并发放当前版本时装","管理 115 级技能进化与强化","调整账号共享的晶体与材料","查询任务进度，管理已接受任务","查看附件，发送游戏内邮件","查看操作记录与完整存档备份"};
 void ResetSelectedItem(){selectedArtGeneration++;selectedItem.NameText="尚未选择物品";selectedItem.Detail="点击物品卡片查看并编辑";selectedItem.Art=new ArtResult();selectedItem.Invalidate();}
 async void ShowSelectedItem(System.Collections.Generic.Dictionary<string,object> row){inventoryBoard.Mark(row);int generation=++selectedArtGeneration;uint id=Convert.ToUInt32(row["template"]);selectedItem.NameText=Program.Str(row,"name");selectedItem.Detail=ContainerText(Program.Str(row,"container"))+"  /  槽位 "+Program.Str(row,"slot")+"  /  ID "+id;selectedItem.Art=new ArtResult();selectedItem.Invalidate();var art=await Art(id);if(IsDisposed||generation!=selectedArtGeneration)return;selectedItem.Art=art;selectedItem.Invalidate();}
 void ArrangeCards(FlowLayoutPanel host){Action arrange=()=>{if(host.Controls.Count==0)return;int gap=host.Controls[0].Margin.Horizontal;int available=Math.Max(1,host.ClientSize.Width-host.Padding.Horizontal-SystemInformation.VerticalScrollBarWidth-2);int minimum=Math.Max(250,Font.Height*18);int columns=Math.Max(1,Math.Min(3,available/(minimum+gap)));int width=Math.Max(100,available/columns-gap);foreach(Control c in host.Controls){c.Width=width;c.PerformLayout();}};host.SizeChanged+=(s,e)=>arrange();host.HandleCreated+=(s,e)=>arrange();host.FontChanged+=(s,e)=>arrange();}
 void UpdatePageHeading(int index){pageHeading.Text=workspace.TabPages[index].Text;pageDescription.Text=index<PageHints.Length?PageHints[index]:"开服期间只读查看全部账号和角色 · 修改存档需进入维护";}
 static void LayoutTree(Control control){control.PerformLayout();foreach(Control child in control.Controls){child.CreateControl();LayoutTree(child);}}
 public void PreviewPage(int index){SelectPage(index);LayoutTree(this);}
 readonly System.Collections.Generic.Dictionary<Control,Font> previewFonts=new System.Collections.Generic.Dictionary<Control,Font>();
 void RememberPreviewFonts(Control c){previewFonts[c]=c.Font;foreach(Control child in c.Controls)RememberPreviewFonts(child);}
 public void PreviewFontScale(double factor){if(!preview)return;if(previewFonts.Count==0)RememberPreviewFonts(this);SuspendLayout();foreach(var pair in previewFonts)pair.Key.Font=new Font(pair.Value.FontFamily,(float)(pair.Value.Size*factor),pair.Value.Style);ResumeLayout(true);LayoutTree(this);}
 public void VerifyPropertiesLayout(){
  SelectPage(0);LayoutTree(this);
  foreach(var nav in navButtons)if(TextRenderer.MeasureText(nav.Text,nav.Font).Width>nav.Width-54)throw new Exception("Navigation caption clipped: "+nav.Text);
  if(pageDescription.Bottom>pageDescription.Parent.ClientSize.Height)throw new Exception("Page description clipped");
  foreach(Control card in workspace.SelectedTab.Controls[0].Controls){
   var stack=card.Controls[0];
   foreach(Control child in stack.Controls)if(child.Bottom>stack.ClientSize.Height||child.Right>stack.ClientSize.Width)throw new Exception("Card content clipped: "+child.GetType().Name+" "+child.Text+" bounds="+child.Bounds+" parent="+stack.ClientSize+" card="+card.Size);
   if(stack.Bottom>card.ClientSize.Height-card.Padding.Bottom)throw new Exception("Card footer clipped");
  }
 }
 public async Task ExportDesign(string directory){
  // A finite read-only render harness. It renders only this form's own controls;
  // no screen capture, operating-system input, player DB, or server API is used.
  Directory.CreateDirectory(directory);await Task.Delay(150);await Art(3037);await Art(3036);await Art(3326);await Art(3038);await Task.Delay(100);if(inventoryRows.Count>0){SelectTagged(inventoryGrid,inventoryRows[0]);ShowSelectedItem(inventoryRows[0]);}foreach(var box in vpEvolve.Concat(vpEnhance)){box.Items.Add(new VPOption{Choice=3,Text="未选择"});box.SelectedIndex=0;}
  foreach(var size in new[]{new Size(1380,940),new Size(1120,720)}){Size=size;for(int i=0;i<workspace.TabPages.Count;i++){SelectPage(i);LayoutTree(this);await Task.Delay(40);using(var bitmap=new Bitmap(Width,Height)){DrawToBitmap(bitmap,new Rectangle(0,0,Width,Height));bitmap.Save(Path.Combine(directory,"gm-"+size.Width+"-"+i.ToString("00")+".png"));}}}
  File.WriteAllText(Path.Combine(directory,"render.json"),Program.Json.Serialize(new{version="1.4.5",edition=Program.Cloud?"cloud":"local",pages=workspace.TabPages.Count,sizes=new[]{"1380x940","1120x720"},player_data_accessed=false,api_requests=0}));
 }
}
