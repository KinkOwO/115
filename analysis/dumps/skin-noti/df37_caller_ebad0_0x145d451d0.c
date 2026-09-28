// caller_ebad0_0x145d451d0

__int64 __fastcall sub_145D451D0(__int64 a1, unsigned int a2)
{
  void (__fastcall ***v4)(_QWORD); // r14
  unsigned int v5; // edi
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  unsigned int v9; // eax
  __int64 v10; // rcx
  __int64 v11; // rax
  unsigned __int8 v12; // r15
  __int64 v13; // rcx
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rax
  __int64 v17; // rsi
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  unsigned int v22; // eax
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rcx
  __int64 v26; // rax
  unsigned int v27; // eax
  unsigned int v28; // eax
  unsigned __int8 v29; // al
  __int64 v30; // rcx

  v4 = 0;
  v5 = 0;
  if ( (unsigned int)sub_145CD39F0(a1, 1) == a2 )
  {
    v6 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v7 = sub_146E8BA20(1472);
      if ( v7 )
        v8 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v7);
      else
        v8 = 0;
      qword_14E638F28 = (__int64)v8;
      (**v8)(v8);
      v6 = qword_14E638F28;
    }
    v9 = sub_1444EA8A0(v6);
  }
  else
  {
    if ( !(unsigned __int8)sub_145CF7C40(a1, a2) )
      goto LABEL_15;
    if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4832LL))(a1) == 3
      && (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4848LL))(a1) == 3
      && a2 == 247 )
    {
      v5 = 100000;
      goto LABEL_15;
    }
    v11 = sub_140764510(v10);
    v9 = sub_1444EBAD0(v11);
  }
  v5 = v9;
LABEL_15:
  v12 = 0;
  v13 = qword_14E634418;
  if ( !qword_14E634418 )
  {
    v14 = sub_146E8BA20(192);
    if ( v14 )
      v15 = (void (__fastcall ***)(_QWORD))sub_144D97100(v14);
    else
      v15 = 0;
    qword_14E634418 = (__int64)v15;
    (**v15)(v15);
    v13 = qword_14E634418;
  }
  v16 = sub_144D9D8E0(v13);
  v17 = v16;
  if ( v16
    && ((*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v16 + 120LL))(v16)
     || (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v17 + 448LL))(v17)
     || (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v17 + 112LL))(v17)) )
  {
    v18 = sub_145F0BA60(qword_14E683C08);
    if ( v18 )
    {
      v19 = sub_145EFFF10(v18);
      if ( v19 )
      {
        v12 = sub_144507FB0(v19, a1);
        if ( v12 )
        {
          if ( (unsigned int)sub_145CD39F0(a1, 1) == a2 )
          {
            v21 = sub_140764510(v20);
            v22 = sub_1444EA9D0(v21, v12);
          }
          else
          {
            if ( !(unsigned __int8)sub_145CF7C40(a1, a2) )
              goto LABEL_32;
            v24 = sub_140764510(v23);
            v22 = sub_1444EBB90(v24, v12);
          }
          v5 = v22;
        }
      }
    }
  }
LABEL_32:
  if ( (unsigned __int8)sub_1431DD4E0(0) )
  {
    v25 = qword_14E636840;
    if ( !qword_14E636840 )
    {
      v26 = sub_146E8BA20(40);
      if ( v26 )
        v4 = (void (__fastcall ***)(_QWORD))sub_1455D9A90(v26);
      qword_14E636840 = (__int64)v4;
      (**v4)(v4);
      v25 = qword_14E636840;
    }
    if ( !(unsigned __int8)sub_1455DC1F0(v25, 23) )
    {
      v27 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 4832LL))(a1);
      v28 = sub_1431DD4D0(v27);
      if ( v28 != -1 )
        v5 = v28;
    }
  }
  (*(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)qword_14E683C68 + 16LL))(qword_14E683C68, 146, 2);
  if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 8 )
    v29 = sub_145F4C000();
  else
    v29 = sub_145F12D90(qword_14E683C20);
  sub_1466642E0(qword_14E683C68, v29);
  sub_146664340(qword_14E683C68, v5);
  sub_1466642E0(qword_14E683C68, v12);
  LOBYTE(v30) = 1;
  return sub_14665AA60(v30, 0);
}

