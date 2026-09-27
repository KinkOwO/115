// holder_sub_145F04E30_0x145f04e30

__int64 __fastcall sub_145F04E30(__int64 a1, __int64 a2)
{
  int v4; // ebp
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  __int64 v7; // rbx
  int v8; // edi
  int v9; // eax
  __int64 v10; // rdx
  __int64 v11; // rcx
  __int64 v12; // rdx
  volatile signed __int32 *v13; // rcx
  __int64 v14; // rcx
  unsigned int v15; // eax
  __int64 result; // rax
  int v17; // r8d
  int v18; // ecx
  __int64 v19; // rax
  __int64 v20; // rbx
  __int64 v21; // rax
  __int64 v22; // rdi
  __int64 v23; // rax
  __int64 v24; // rcx
  __int64 v25; // rax
  __int64 v26; // rcx
  __int64 v27; // rax
  __int64 v28; // rdi
  void (__fastcall *v29)(__int64, __int64); // rsi
  __int64 v30; // rax
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // rcx
  __int64 v34; // rax
  __int64 v35; // rcx
  __int64 v36; // rdx
  __int64 v37; // rax
  __int64 v38; // rcx
  __int64 v39; // rdi
  __int64 v40; // rdx
  __int64 v41; // rcx
  __int64 v42; // rax
  __int64 v43; // rdi
  __int64 v44; // rax
  void (__fastcall ***v45)(_QWORD); // rcx
  __int64 v46; // rax
  __int64 v47; // rcx
  __int64 v48; // rax
  __int64 v49; // rax
  __int64 v50; // rcx
  __int64 v51; // rax
  __int64 v52; // rcx
  __int64 v53; // rax
  __int64 v54; // rdi
  __int64 v55; // rcx
  __int64 v56; // rax
  void (__fastcall ***v57)(_QWORD); // rcx
  __int64 v58; // rax
  __int64 v59; // rcx
  __int64 v60; // rax
  __int64 v61; // rcx
  __int64 v62; // rax
  __int64 v63; // rcx
  __int64 v64; // rax
  __int64 v65; // rcx
  __int64 v66; // rax
  __int64 v67; // rcx
  __int64 v68; // rax
  __int64 v69; // rcx
  __int64 v70; // rax
  __int64 v71; // rcx
  __int64 v72; // rax
  __int64 v73; // rcx
  __int64 v74; // rax
  __int64 v75; // rcx
  __int64 v76; // rax
  __int64 v77; // rcx
  __int64 v78; // rax
  __int64 v79; // rcx
  __int64 v80; // rax
  __int64 v81; // rcx
  __int64 v82; // rax
  __int64 v83; // rcx
  __int64 v84; // rax
  __int64 v85; // rcx
  __int64 v86; // [rsp+58h] [rbp-40h] BYREF
  __int64 v87; // [rsp+60h] [rbp-38h]
  __int64 v88; // [rsp+68h] [rbp-30h]
  void *retaddr; // [rsp+98h] [rbp+0h]
  __int64 v90; // [rsp+A0h] [rbp+8h] BYREF
  int v91; // [rsp+A8h] [rbp+10h] BYREF

  if ( !a2 )
  {
    v4 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v5 = sub_146E8BA20(72);
      v90 = v5;
      if ( v5 )
        v6 = (void (__fastcall ***)(_QWORD))sub_146E93360(v5);
      else
        v6 = 0;
      qword_14E6343D0 = (__int64)v6;
      (**v6)(v6);
      v4 = qword_14E6343D0;
    }
    v7 = sub_146E8C7D0(&unk_14A986B80);
    v8 = sub_146E8C7D0(&unk_14A986B48);
    v9 = sub_146E8C7D0(&unk_14A986AE0);
    sub_146E938E0(v4, 0, v9, v8, 2366, (__int64)&qword_14EF2CB68, v7);
  }
  v10 = a2 + 48;
  if ( !a2 )
    v10 = 0;
  sub_146EA47F0(&v86, v10);
  v11 = 0;
  v12 = 0;
  if ( v87 )
  {
    v11 = v86;
    v12 = v87;
    _InterlockedIncrement((volatile signed __int32 *)(v87 + 12));
  }
  *(_QWORD *)(a1 + 2072) = v11;
  v13 = *(volatile signed __int32 **)(a1 + 2080);
  *(_QWORD *)(a1 + 2080) = v12;
  if ( v13 && _InterlockedExchangeAdd(v13 + 3, 0xFFFFFFFF) == 1 )
    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v13 + 8LL))(v13);
  *(_QWORD *)(a1 + 2088) = v88;
  v14 = v87;
  if ( v87 && _InterlockedExchangeAdd((volatile signed __int32 *)(v87 + 12), 0xFFFFFFFF) == 1 )
    (*(void (__fastcall **)(__int64))(*(_QWORD *)v14 + 8LL))(v14);
  if ( a2 )
  {
    sub_145D369B0(a2, *(unsigned __int16 *)(a1 + 112));
    sub_145B96B50(a2, 5);
    v15 = sub_145EFAEA0(a2);
    sub_145B99610(a2, v15);
  }
  result = sub_145F033C0(a1);
  if ( !(_BYTE)result )
  {
    sub_146E920A0(&dword_14EF2CA00, &v90);
    v17 = v90;
    v18 = v90 + dword_14EF2CA00 + 196;
    if ( dword_14EF2CA04 && v18 && dword_14EF2CA04 != v18 && retaddr )
    {
      sub_146D89B40(retaddr, &dword_14EF2CA00);
      v17 = v90;
    }
    result = *(unsigned __int16 *)(a1 + 112);
    if ( (_DWORD)result == v17 )
    {
      if ( !qword_14EF2CA70 || (v19 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
        v19 = 0;
      v20 = v19 - 48;
      if ( !v19 )
        v20 = 0;
      sub_1401F10B0(&qword_14EF2CA68, a1 + 2072);
      result = qword_14EF2CA70;
      if ( qword_14EF2CA70 && *(_DWORD *)(qword_14EF2CA70 + 8) )
      {
        result = qword_14EF2CA78 - 48;
        if ( !qword_14EF2CA78 )
          result = 0;
        if ( result )
        {
          result = v20 + 48;
          if ( !v20 )
            result = 0;
          if ( qword_14EF2CA78 != result )
          {
            v21 = sub_146E8BA20(240);
            v90 = v21;
            if ( v21 )
              v22 = sub_146D7EAE0(v21);
            else
              v22 = 0;
            if ( !qword_14EF2CA70 || (v23 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v23 = 0;
            v24 = v23 - 48;
            if ( !v23 )
              v24 = 0;
            sub_145CEAA10(v24, v22, 0);
            if ( !qword_14EF2CA70 || (v25 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v25 = 0;
            v26 = v25 - 48;
            if ( !v25 )
              v26 = 0;
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v26 + 9032LL))(v26, v22);
            if ( !qword_14EF2CA70 || (v27 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v27 = 0;
            v28 = v27 - 48;
            if ( !v27 )
              v28 = 0;
            v29 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v28 + 9040LL);
            v30 = sub_146E8BA20(400);
            v90 = v30;
            if ( v30 )
              v31 = sub_145EF18A0(v30, 512);
            else
              v31 = 0;
            v29(v28, v31);
            if ( !qword_14EF2CA70 || (v32 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v32 = 0;
            v33 = v32 - 48;
            if ( !v32 )
              v33 = 0;
            v34 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v33 + 8976LL))(v33);
            if ( !qword_14EF2CA70 || (v35 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v35 = 0;
            v36 = v35 - 48;
            if ( !v35 )
              v36 = 0;
            sub_145EEB220(v34, v36);
            if ( !qword_14EF2CA70 || (v37 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v37 = 0;
            v38 = v37 - 48;
            if ( !v37 )
              v38 = 0;
            v39 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v38 + 8976LL))(v38);
            if ( !qword_14EF2CA70 || (v40 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v40 = 0;
            v41 = v40 - 48;
            if ( !v40 )
              v41 = 0;
            v42 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v41 + 8968LL))(v41);
            sub_142577BB0(v39, v42);
            v43 = qword_14E663548;
            if ( !qword_14E663548 )
            {
              v44 = sub_146E8BA20(792);
              v90 = v44;
              if ( v44 )
                v45 = (void (__fastcall ***)(_QWORD))sub_14601DAA0(v44);
              else
                v45 = 0;
              qword_14E663548 = (__int64)v45;
              (**v45)(v45);
              v43 = qword_14E663548;
            }
            if ( !qword_14EF2CA70 || (v46 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v46 = 0;
            v47 = v46 - 48;
            if ( !v46 )
              v47 = 0;
            v48 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v47 + 8968LL))(v47);
            sub_14602CE30(v43, v48);
            if ( !qword_14EF2CA70 || (v49 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v49 = 0;
            v50 = v49 - 48;
            if ( !v49 )
              v50 = 0;
            sub_145D39040(v50, 0);
            if ( !qword_14EF2CA70 || (v51 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v51 = 0;
            v52 = v51 - 48;
            if ( !v51 )
              v52 = 0;
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v52 + 8712LL))(v52);
            if ( !qword_14EF2CA70 || (v53 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v53 = 0;
            v54 = v53 - 48;
            if ( !v53 )
              v54 = 0;
            sub_144EE56E0();
            v55 = qword_14E63AE60;
            if ( !qword_14E63AE60 )
            {
              v56 = sub_146E8BA20(336);
              v90 = v56;
              if ( v56 )
                v57 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v56);
              else
                v57 = 0;
              qword_14E63AE60 = (__int64)v57;
              (**v57)(v57);
              v55 = qword_14E63AE60;
            }
            v91 = sub_14109DE10(v55);
            sub_145D83900(v54, &v91);
            if ( v20 )
            {
              if ( sub_145CD4020(v20) )
                qword_14E683C80 = sub_145CD4020(v20);
              if ( sub_145CD4000(v20) )
                qword_14E683CB0 = sub_145CD4000(v20);
              if ( sub_145CD4010(v20) )
                qword_14E683CB8 = sub_145CD4010(v20);
              if ( sub_145CD4030(v20) )
                qword_14E683CC0 = sub_145CD4030(v20);
            }
            if ( !qword_14EF2CA70 || (v58 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v58 = 0;
            v59 = v58 - 48;
            if ( !v58 )
              v59 = 0;
            sub_145D2D4F0(v59, qword_14E683C80);
            if ( !qword_14EF2CA70 || (v60 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v60 = 0;
            v61 = v60 - 48;
            if ( !v60 )
              v61 = 0;
            sub_145D2D4C0(v61, qword_14E683CB0);
            if ( !qword_14EF2CA70 || (v62 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v62 = 0;
            v63 = v62 - 48;
            if ( !v62 )
              v63 = 0;
            sub_145D2D4D0(v63, qword_14E683CB8);
            if ( !qword_14EF2CA70 || (v64 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v64 = 0;
            v65 = v64 - 48;
            if ( !v64 )
              v65 = 0;
            sub_145D2D500(v65, qword_14E683CC0);
            if ( !qword_14EF2CA70 || (v66 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v66 = 0;
            v67 = v66 - 48;
            if ( !v66 )
              v67 = 0;
            if ( sub_145CE07E0(v67) )
            {
              if ( !qword_14EF2CA70 || (v68 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
                v68 = 0;
              v69 = v68 - 48;
              if ( !v68 )
                v69 = 0;
              qword_14E683C80 = sub_145CE07E0(v69);
            }
            if ( !qword_14EF2CA70 || (v70 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v70 = 0;
            v71 = v70 - 48;
            if ( !v70 )
              v71 = 0;
            if ( sub_145CD39E0(v71) )
            {
              if ( !qword_14EF2CA70 || (v72 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
                v72 = 0;
              v73 = v72 - 48;
              if ( !v72 )
                v73 = 0;
              qword_14E683CB0 = sub_145CD39E0(v73);
            }
            if ( !qword_14EF2CA70 || (v74 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v74 = 0;
            v75 = v74 - 48;
            if ( !v74 )
              v75 = 0;
            if ( sub_145CD78D0(v75) )
            {
              if ( !qword_14EF2CA70 || (v76 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
                v76 = 0;
              v77 = v76 - 48;
              if ( !v76 )
                v77 = 0;
              qword_14E683CB8 = sub_145CD78D0(v77);
            }
            if ( !qword_14EF2CA70 || (v78 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v78 = 0;
            v79 = v78 - 48;
            if ( !v78 )
              v79 = 0;
            if ( sub_145CE43B0(v79) )
            {
              if ( !qword_14EF2CA70 || (v80 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
                v80 = 0;
              v81 = v80 - 48;
              if ( !v80 )
                v81 = 0;
              qword_14E683CC0 = sub_145CE43B0(v81);
            }
            if ( qword_14E683C80 )
              sub_145AD0C40();
            if ( !qword_14EF2CA70 || (v82 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v82 = 0;
            v83 = v82 - 48;
            if ( !v82 )
              v83 = 0;
            sub_145CAFEE0(v83);
            if ( !qword_14EF2CA70 || (v84 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
              v84 = 0;
            v85 = v84 - 48;
            if ( !v84 )
              v85 = 0;
            return sub_145CB0210(v85);
          }
        }
      }
    }
  }
  return result;
}

