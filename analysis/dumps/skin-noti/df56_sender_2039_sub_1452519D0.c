// sender_2039_sub_1452519D0

__int64 __fastcall sub_1452519D0(__int64 a1, __int64 a2, __int64 a3, __int64 a4)
{
  int v4; // r14d
  int v5; // edi
  bool v6; // bl
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 *v9; // rdi
  char v10; // r8
  int v11; // r9d
  __int64 v12; // rax
  char *v13; // rax
  signed __int64 v14; // rdx
  char v15; // cl
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // r8
  bool v19; // di
  unsigned __int64 v20; // rdx
  __int64 v21; // rcx
  __int64 v22; // rax
  int v23; // eax
  __int64 v24; // rcx
  __int64 v25; // rax
  __int64 v26; // rdx
  __int64 v27; // rcx
  int v28; // esi
  __int64 v29; // rbx
  int v30; // edi
  int v31; // eax
  __int64 v32; // rcx
  int v33; // esi
  __int64 v34; // rbx
  int v35; // edi
  int v36; // eax
  char v37; // r12
  char v38; // si
  __int64 v39; // rax
  __int64 v40; // rdx
  __int64 v41; // rcx
  __int64 v42; // rax
  int v43; // eax
  __int64 v44; // rax
  __int64 v45; // rbx
  __int64 v46; // rax
  __int64 v47; // rdx
  __int64 v48; // rcx
  __int64 v49; // rax
  int v50; // eax
  __int64 v51; // rax
  __int64 v52; // rax
  __int64 v53; // rdx
  __int64 v54; // rcx
  __int64 v55; // rax
  int v56; // eax
  __int64 v57; // rax
  __int64 v58; // rbx
  __int64 v59; // rbx
  unsigned __int64 v60; // rdx
  __int64 v61; // rcx
  __int64 v62; // rax
  __int128 *v63; // rbx
  __int64 v64; // rax
  __int64 v65; // rax
  __int64 v66; // rax
  unsigned __int64 v67; // rdx
  __int64 v68; // rcx
  __int64 v69; // rax
  __int64 v70; // rax
  __int64 v71; // rax
  __int64 v72; // rax
  int v74; // [rsp+28h] [rbp-59h]
  _BYTE v75[4]; // [rsp+48h] [rbp-39h] BYREF
  unsigned __int16 v76; // [rsp+4Ch] [rbp-35h] BYREF
  int v77; // [rsp+50h] [rbp-31h] BYREF
  unsigned int v78; // [rsp+54h] [rbp-2Dh]
  __int64 v79; // [rsp+58h] [rbp-29h]
  __int128 v80; // [rsp+60h] [rbp-21h] BYREF
  __int128 v81; // [rsp+70h] [rbp-11h]
  _QWORD v82[3]; // [rsp+88h] [rbp+7h] BYREF
  unsigned __int64 v83; // [rsp+A0h] [rbp+1Fh]
  void *retaddr; // [rsp+E0h] [rbp+5Fh]

  v79 = -2;
  v4 = (unsigned __int16)a3;
  v5 = 0;
  v78 = 0;
  if ( !(_BYTE)a2 )
  {
    switch ( (unsigned __int16)a3 )
    {
      case 2u:
        v28 = sub_14021A860(a1, a2, a3, a4, v74);
        v29 = sub_14723C170(689);
        v30 = sub_146E8C7D0(&unk_14A79C050);
        v31 = sub_146E8C7D0(&unk_14A79BDF0);
        sub_146E939E0(v28, 0, v31, v30, 3247, (__int64)&qword_14E66C128, v29);
LABEL_88:
        LOBYTE(v74) = 1;
        LOBYTE(a4) = 1;
        sub_146ADFC80(v4, 6, 1, a4, v74, 1);
        goto LABEL_89;
      case 0x13u:
        v32 = 690;
        break;
      case 0x15u:
        v33 = sub_14021A860(a1, a2, a3, a4, v74);
        v34 = sub_14723C170(691);
        v35 = sub_146E8C7D0(&unk_14A79C050);
        v36 = sub_146E8C7D0(&unk_14A79BDF0);
        sub_146E939E0(v33, 0, v36, v35, 3258, (__int64)&qword_14E66C128, v34);
        goto LABEL_88;
      case 0x16u:
        v32 = 692;
        break;
      case 0x28u:
        v32 = 693;
        break;
      case 0x24u:
        v32 = 25005;
        break;
      case 0x76u:
        goto LABEL_88;
      default:
        switch ( (_WORD)a3 )
        {
          case 0xEB:
            v32 = 49100;
            break;
          case 0xBF:
            if ( !qword_14E683C78 )
              goto LABEL_88;
            v32 = 100002098;
            break;
          case 0x140:
            v37 = 0;
            v38 = 0;
            v39 = sub_14021A2C0(a1, a2);
            if ( (unsigned __int8)sub_145695000(v39, 475) == 1 )
            {
              v42 = sub_14021A2C0(v41, v40);
              v43 = sub_145693930(v42, 475);
              v44 = sub_148AA307C(v43, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DCB48D8, 0);
              v45 = v44;
              if ( v44 )
              {
                if ( (unsigned int)sub_140696260(v44) != -1 || *(_DWORD *)(sub_140BFB400(v45) + 16) != 17 )
                  v38 = 1;
              }
            }
            v46 = sub_14021A2C0(v41, v40);
            if ( (unsigned __int8)sub_145695000(v46, 516) == 1 )
            {
              v49 = sub_14021A2C0(v48, v47);
              v50 = sub_145693930(v49, 516);
              v51 = sub_148AA307C(v50, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DCB4900, 0);
              if ( v51 )
              {
                if ( *(_DWORD *)(sub_14017AED0(v51) + 16) != 17 )
                  v38 = 1;
              }
            }
            *(_QWORD *)&v80 = 0;
            *(_QWORD *)&v81 = 0;
            *((_QWORD *)&v81 + 1) = 7;
            v52 = sub_14021A2C0(v48, v47);
            if ( (unsigned __int8)sub_145695000(v52, 802) == 1 )
            {
              v55 = sub_14021A2C0(v54, v53);
              v56 = sub_145693930(v55, 802);
              v57 = sub_148AA307C(v56, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DCB4930, 0);
              v58 = v57;
              if ( v57 )
              {
                if ( (unsigned int)sub_140696260(v57) != -1 || *(_DWORD *)(sub_1427F9240(v58) + 16) != 17 )
                {
                  v37 = 1;
                  v59 = sub_1427F9B90(v58, v82);
                  if ( &v80 != (__int128 *)v59 )
                  {
                    *(_QWORD *)&v81 = 0;
                    *((_QWORD *)&v81 + 1) = 7;
                    LOWORD(v80) = 0;
                    v80 = *(_OWORD *)v59;
                    v81 = *(_OWORD *)(v59 + 16);
                    *(_QWORD *)(v59 + 16) = 0;
                    *(_QWORD *)(v59 + 24) = 7;
                    *(_WORD *)v59 = 0;
                  }
                  if ( v83 >= 8 )
                  {
                    v60 = 2 * v83 + 2;
                    v61 = v82[0];
                    if ( v60 >= 0x1000 )
                    {
                      v60 = 2 * v83 + 41;
                      v61 = *(_QWORD *)(v82[0] - 8LL);
                      if ( (unsigned __int64)(v82[0] - v61 - 8) > 0x1F )
                        sub_148AAF304(v61, v60);
                    }
                    sub_146E9F3A0(v61, v60);
                  }
                  v82[2] = 0;
                  v83 = 7;
                  LOWORD(v82[0]) = 0;
                }
              }
            }
            if ( v38 == 1 )
            {
              if ( qword_14E683C78 )
              {
                v62 = sub_14723C170(100087122);
                sub_14668C520(qword_14E683C78, 2875, v62, 0);
              }
            }
            else if ( v37 == 1 )
            {
              if ( qword_14E683C78 )
              {
                if ( *((_QWORD *)&v81 + 1) < 8u )
                  v63 = &v80;
                else
                  v63 = (__int128 *)v80;
                v64 = sub_14723C170(100086018);
                v65 = sub_146E8CF20(v82, v64, v63);
                v5 = 1;
                v78 = 1;
                v66 = sub_14014F430(v65);
                sub_14668C520(qword_14E683C78, 2875, v66, 0);
              }
              if ( (v5 & 1) != 0 )
              {
                v78 = v5 & 0xFFFFFFFE;
                sub_146E8C910(v82);
              }
            }
            if ( *((_QWORD *)&v81 + 1) >= 8u )
            {
              v67 = 2LL * *((_QWORD *)&v81 + 1) + 2;
              v68 = v80;
              if ( v67 >= 0x1000 )
              {
                v67 = 2LL * *((_QWORD *)&v81 + 1) + 41;
                v68 = *(_QWORD *)(v80 - 8);
                if ( (unsigned __int64)(v80 - v68 - 8) > 0x1F )
                  sub_148AAF304(v68, v67);
              }
              sub_146E9F3A0(v68, v67);
            }
            *(_QWORD *)&v81 = 0;
            *((_QWORD *)&v81 + 1) = 7;
            LOWORD(v80) = 0;
            goto LABEL_88;
          default:
            if ( (unsigned __int16)a3 == 7 )
            {
              if ( !qword_14E683C78 )
                goto LABEL_88;
              v32 = 100088493;
            }
            else
            {
              if ( (_WORD)a3 != 444 || !qword_14E683C78 )
                goto LABEL_88;
              v32 = 100086328;
            }
            break;
        }
        break;
    }
    v69 = sub_14723C170(v32);
    sub_14668C520(qword_14E683C78, 2875, v69, 0);
    goto LABEL_88;
  }
  v6 = 0;
  v75[0] = 0;
  sub_146EA09F0(v75);
  if ( v75[0] )
  {
    v7 = sub_141329A00(qword_14E66C090);
    sub_14023AF60(v7, 0);
  }
  v76 = 0;
  sub_146EA1920(&v76);
  v8 = sub_141329A00(qword_14E66C090);
  v9 = (__int64 *)sub_14021ADF0(v8, v76, 0xFFFFFFFFLL);
  if ( v9 )
  {
    sub_1471B21D0(&v80);
    sub_146E920A0(&dword_14E682A58, &v77);
    v10 = v77;
    v11 = v77 + dword_14E682A58 + 196;
    if ( dword_14E682A5C && v11 && dword_14E682A5C != v11 && retaddr )
    {
      sub_146D89B40(retaddr, &dword_14E682A58);
      v10 = v77;
    }
    LOBYTE(v80) = v10;
    v12 = sub_14014F430(v9);
    v13 = (char *)sub_146E90780(v12);
    v14 = (char *)&v80 + 8 - v13;
    do
    {
      v15 = *v13;
      v13[v14] = *v13;
      ++v13;
    }
    while ( v15 );
    sub_144F67030(qword_14E683D40, &v80, 1);
  }
  v16 = sub_141329A00(qword_14E66C090);
  sub_140235650(v16, v76);
  if ( sub_142AF3AD0() && (unsigned __int8)sub_142AF5F20() == 1 )
  {
    v17 = *v9;
    *(_QWORD *)&v80 = 0;
    *(_QWORD *)&v81 = 0;
    *((_QWORD *)&v81 + 1) = 7;
    v18 = -1;
    do
      ++v18;
    while ( *(_WORD *)(v17 + 2 * v18) );
    sub_14014C8D0(&v80, v17);
    v19 = (unsigned __int8)sub_142AF5B80(&v80) == 1;
    if ( *((_QWORD *)&v81 + 1) >= 8u )
    {
      v20 = 2LL * *((_QWORD *)&v81 + 1) + 2;
      v21 = v80;
      if ( v20 >= 0x1000 )
      {
        v20 = 2LL * *((_QWORD *)&v81 + 1) + 41;
        v21 = *(_QWORD *)(v80 - 8);
        if ( (unsigned __int64)(v80 - v21 - 8) > 0x1F )
          sub_148AAF304(v21, v20);
      }
      sub_146E9F3A0(v21, v20);
    }
    *(_QWORD *)&v81 = 0;
    *((_QWORD *)&v81 + 1) = 7;
    LOWORD(v80) = 0;
    v6 = v19;
  }
  if ( !(unsigned __int8)sub_142AF5F20() || !v6 )
  {
    v22 = sub_14723C170(688);
    sub_14668C520(qword_14E683C78, 2875, v22, 0);
  }
  v23 = sub_146E8C7D0(&unk_14975AE30);
  sub_145A31380(v23, -1, 0, 0, -1, -1, 0);
  v25 = sub_146D74000(v24);
  sub_146D746E0(v25, 637);
  sub_146D75AF0(v27, v26);
  sub_1401FDEE0();
LABEL_89:
  v70 = sub_141329A00(qword_14E66C090);
  sub_14023DFD0(v70, 0);
  v71 = sub_141329A00(qword_14E66C090);
  sub_14023DFC0(v71, 0);
  v72 = sub_141329A00(qword_14E66C090);
  return sub_140228D00(v72);
}

